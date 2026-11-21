<script setup lang="ts">
// CCMTV 授权控制台（inhouse:read / inhouse:write 超管专属）：
// 包含：全局硬授权启停、客户创建时间与备注编辑/删除、单客户封禁/恢复、
// Loon / 小火箭激活状态（首次激活时间 + 最近核验时间）、待激活码随时查看链接与配置下载、授权审计日志抽屉。
import { computed, h, onMounted, reactive, ref } from "vue";
import {
  NAlert,
  NButton,
  NCollapse,
  NCollapseItem,
  NDataTable,
  NDrawer,
  NDrawerContent,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NSpace,
  NTag,
} from "naive-ui";
import type { DataTableColumns } from "naive-ui";
import {
  createCCMTVCustomer,
  deleteCCMTVCustomer,
  fetchCCMTVState,
  issueCCMTVCode,
  setCCMTVCustomerStatus,
  setCCMTVGlobal,
  updateCCMTVCustomer,
  type CCMTVAuditItem,
  type CCMTVCustomer,
  type CCMTVInstallSlot,
  type CCMTVIssueResult,
  type CCMTVPlatform,
  type CCMTVState,
} from "@/service/api";
import { checkAuth } from "@/directives";
import FilterTabs from "@/components/common/filter-tabs.vue";

defineOptions({ name: "CCMTVInhouseTab" });

const loading = ref(false);
const globalSaving = ref(false);
const globalEnabled = ref(true);
const publicOrigin = ref("");
const customers = ref<CCMTVCustomer[]>([]);
const audits = ref<CCMTVAuditItem[]>([]);

const statusFilter = ref<"all" | "enabled" | "paused">("all");
const keyword = ref("");

const canWrite = () => checkAuth("inhouse:write");

function isCustomerEnabled(row: CCMTVCustomer): boolean {
  return Boolean(row.enabled);
}

const statusCounts = computed(() => {
  let enabledCount = 0;
  let pausedCount = 0;
  for (const row of customers.value) {
    if (isCustomerEnabled(row)) enabledCount += 1;
    else pausedCount += 1;
  }
  return {
    all: customers.value.length,
    enabled: enabledCount,
    paused: pausedCount,
  };
});

const statusTabs = [
  { label: "全部", value: "all", type: "default" as const },
  { label: "已启用", value: "enabled", type: "success" as const },
  { label: "已暂停", value: "paused", type: "error" as const },
];

const filteredCustomers = computed(() => {
  const q = keyword.value.trim().toLowerCase();
  return customers.value.filter((row) => {
    if (statusFilter.value === "enabled" && !isCustomerEnabled(row)) return false;
    if (statusFilter.value === "paused" && isCustomerEnabled(row)) return false;
    if (q) {
      const hitMeta =
        row.label.toLowerCase().includes(q) ||
        row.id.toLowerCase().includes(q) ||
        String(row.installs?.loon?.code_plain || "").toLowerCase().includes(q) ||
        String(row.installs?.shadowrocket?.code_plain || "").toLowerCase().includes(q);
      if (!hitMeta) return false;
    }
    return true;
  });
});

function applyState(state: CCMTVState) {
  globalEnabled.value = Boolean(state.enabled);
  publicOrigin.value = state.public_origin || "";
  customers.value = Array.isArray(state.customers) ? state.customers : [];
  if (Array.isArray(state.audits)) {
    audits.value = state.audits;
  }
}

async function load() {
  loading.value = true;
  try {
    const { data, error } = await fetchCCMTVState();
    if (!error && data) {
      applyState(data);
    }
  } finally {
    loading.value = false;
  }
}

async function toggleGlobal(target: boolean) {
  if (globalSaving.value) return;
  globalSaving.value = true;
  try {
    const { data, error } = await setCCMTVGlobal(target);
    if (!error && data) {
      applyState(data);
      window.$message?.success("状态已保存，客户端将在下次核验时应用。");
    }
  } finally {
    globalSaving.value = false;
  }
}

const togglingRow = reactive<Record<string, boolean>>({});
async function toggleCustomerStatus(row: CCMTVCustomer, nextEnabled: boolean) {
  if (togglingRow[row.id]) return;
  togglingRow[row.id] = true;
  try {
    const { data, error } = await setCCMTVCustomerStatus(row.id, nextEnabled);
    if (!error && data) {
      applyState(data);
      window.$message?.success("状态已保存，客户端将在下次核验时应用。");
    }
  } finally {
    togglingRow[row.id] = false;
  }
}

// ── 添加客户弹窗 ──
const showAddModal = ref(false);
const adding = ref(false);
const newLabel = ref("");

function openAddModal() {
  newLabel.value = "";
  showAddModal.value = true;
}

async function submitAddCustomer() {
  const label = newLabel.value.trim();
  if (!label || label.length > 80) {
    window.$message?.warning("请输入 1 到 80 个字符的客户备注。");
    return;
  }
  if (adding.value) return;
  adding.value = true;
  try {
    const { data, error } = await createCCMTVCustomer(label);
    if (!error && data) {
      applyState(data);
      showAddModal.value = false;
      window.$message?.success("客户已添加。");
    }
  } finally {
    adding.value = false;
  }
}

// ── 修改客户备注弹窗 ──
const showEditModal = ref(false);
const editing = ref(false);
const editCustomerId = ref("");
const editLabel = ref("");

function openEditModal(row: CCMTVCustomer) {
  editCustomerId.value = row.id;
  editLabel.value = row.label;
  showEditModal.value = true;
}

async function submitEditCustomer() {
  const label = editLabel.value.trim();
  if (!label || label.length > 80) {
    window.$message?.warning("请输入 1 到 80 个字符的客户备注。");
    return;
  }
  if (editing.value) return;
  editing.value = true;
  try {
    const { data, error } = await updateCCMTVCustomer(editCustomerId.value, label);
    if (!error && data) {
      applyState(data);
      showEditModal.value = false;
      window.$message?.success("客户备注已更新。");
    }
  } finally {
    editing.value = false;
  }
}

// ── 删除客户 ──
const deletingRow = reactive<Record<string, boolean>>({});
async function handleDeleteCustomer(row: CCMTVCustomer) {
  if (deletingRow[row.id]) return;
  deletingRow[row.id] = true;
  try {
    const { data, error } = await deleteCCMTVCustomer(row.id);
    if (!error && data) {
      applyState(data);
      window.$message?.success("客户已删除。");
    }
  } finally {
    deletingRow[row.id] = false;
  }
}

// ── 激活码查看 / 签发弹窗 ──
const issuingKey = reactive<Record<string, boolean>>({});
const showIssuedModal = ref(false);
const issuedInfo = ref<CCMTVIssueResult | null>(null);

async function handleIssue(row: CCMTVCustomer, platform: CCMTVPlatform, reuse = false) {
  const key = `${row.id}:${platform}:${reuse ? "view" : "new"}`;
  if (issuingKey[key]) return;
  issuingKey[key] = true;
  try {
    const { data, error } = await issueCCMTVCode(row.id, platform, reuse);
    if (!error && data) {
      issuedInfo.value = data;
      showIssuedModal.value = true;
      if (!reuse) {
        await load();
      }
    }
  } finally {
    issuingKey[key] = false;
  }
}

function platformName(p: CCMTVPlatform): string {
  return p === "loon" ? "Loon" : "小火箭";
}

function formatDateTime(ts?: number | null, withYear = true): string {
  if (!ts) return "-";
  try {
    return new Intl.DateTimeFormat("zh-CN", {
      timeZone: "Asia/Shanghai",
      ...(withYear ? { year: "numeric" } : {}),
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    }).format(new Date(ts));
  } catch {
    return "-";
  }
}

function buildInstallUrl(platform: CCMTVPlatform, code: string): string {
  const origin = (publicOrigin.value || "").replace(/\/+$/, "");
  const ext = platform === "loon" ? "plugin" : "module";
  return `${origin}/install/${platform}/${code}.${ext}`;
}

function slotView(slot?: CCMTVInstallSlot): {
  status: "activated" | "pending" | "idle";
  label: string;
  tagType: "success" | "info" | "default";
  hasSlot: boolean;
  codePlain: string;
} {
  if (!slot) {
    return { status: "idle", label: "未激活", tagType: "default", hasSlot: false, codePlain: "" };
  }
  const codePlain = String(slot.code_plain || "");
  if (slot.activated_at) {
    return { status: "activated", label: "已激活", tagType: "success", hasSlot: true, codePlain };
  }
  if (slot.pending || slot.expires_at) {
    return { status: "pending", label: "待激活", tagType: "info", hasSlot: true, codePlain };
  }
  return { status: "idle", label: "未激活", tagType: "default", hasSlot: true, codePlain };
}

function renderPlatformCell(row: CCMTVCustomer, platform: CCMTVPlatform) {
  const slot = row.installs?.[platform];
  const st = slotView(slot);
  const viewKey = `${row.id}:${platform}:view`;
  const newKey = `${row.id}:${platform}:new`;
  const pname = platformName(platform);

  const buttons: any[] = [];
  if (canWrite()) {
    if (st.codePlain) {
      buttons.push(
        h(
          NButton,
          {
            size: "tiny",
            type: "info",
            secondary: true,
            loading: Boolean(issuingKey[viewKey]),
            onClick: () => handleIssue(row, platform, true),
          },
          { default: () => "配置详情" },
        ),
      );
    }

    if (st.hasSlot) {
      buttons.push(
        h(
          NPopconfirm,
          { onPositiveClick: () => handleIssue(row, platform, false) },
          {
            trigger: () =>
              h(
                NButton,
                { size: "tiny", secondary: true, loading: Boolean(issuingKey[newKey]) },
                { default: () => (st.status === "pending" ? "重发" : "换机或重发") },
              ),
            default: () =>
              st.status === "pending"
                ? `重发将作废当前的待激活 ${pname} 链接并生成新码，确定继续吗？`
                : `重发将立即作废该客户旧的 ${pname} 凭证，确定继续吗？`,
          },
        ),
      );
    } else {
      buttons.push(
        h(
          NButton,
          {
            size: "tiny",
            type: "primary",
            secondary: true,
            loading: Boolean(issuingKey[newKey]),
            onClick: () => handleIssue(row, platform, false),
          },
          { default: () => "生成激活码" },
        ),
      );
    }
  }

  const headerRow = h("div", { class: "flex flex-wrap items-center justify-between gap-6px" }, [
    h(
      NTag,
      { size: "small", type: st.tagType, bordered: false, class: "w-fit" },
      { default: () => st.label },
    ),
    buttons.length ? h("div", { class: "flex flex-wrap items-center gap-4px" }, buttons) : null,
  ]);

  const codeRow = st.codePlain
    ? h("div", { class: "flex flex-wrap items-center gap-4px text-11px" }, [
        h("span", { class: "text-gray-500" }, "激活码:"),
        h(
          "code",
          {
            class: "rounded bg-gray-100 px-5px py-1px font-mono text-11px text-gray-800 dark:bg-gray-800 dark:text-gray-200",
            title: st.codePlain,
          },
          st.codePlain,
        ),
        h(
          NButton,
          {
            size: "tiny",
            text: true,
            type: "primary",
            onClick: () => copyText(st.codePlain, "激活码"),
          },
          { default: () => "复制码" },
        ),
        publicOrigin.value
          ? h(
              NButton,
              {
                size: "tiny",
                text: true,
                type: "info",
                onClick: () => copyText(buildInstallUrl(platform, st.codePlain), `${pname} 导入链接`),
              },
              { default: () => "复制链接" },
            )
          : null,
      ])
    : st.hasSlot
      ? h("div", { class: "text-11px text-gray-400" }, "激活码：旧版生成（未留存明文）")
      : null;

  const timeDetails =
    st.status === "activated" && slot
      ? h("div", { class: "flex flex-wrap items-center gap-8px text-11px text-gray-500 dark:text-gray-400" }, [
          h("span", `首次激活: ${formatDateTime(slot.activated_at, false)}`),
          h("span", `最近核验: ${slot.last_seen_at ? formatDateTime(slot.last_seen_at, false) : "暂无"}`),
        ])
      : null;

  return h("div", { class: "flex flex-col gap-4px" }, [headerRow, codeRow, timeDetails]);
}

const columns = computed<DataTableColumns<CCMTVCustomer>>(() => [
  {
    title: "客户备注",
    key: "label",
    minWidth: 180,
    render: (row) =>
      h("div", { class: "flex flex-col gap-2px" }, [
        h("span", { class: "font-500" }, row.label),
        row.created_at
          ? h("span", { class: "text-11px text-gray-400" }, `创建于 ${formatDateTime(row.created_at, true)}`)
          : null,
      ]),
  },
  {
    title: "授权状态",
    key: "enabled",
    width: 96,
    render: (row) => {
      const enabled = isCustomerEnabled(row);
      const text = enabled ? (globalEnabled.value ? "已启用" : "全局暂停") : "已暂停";
      const type = text === "已启用" ? "success" : enabled ? "warning" : "error";
      return h(NTag, { size: "small", type, bordered: false }, { default: () => text });
    },
  },
  {
    title: "Loon 客户端",
    key: "loon",
    minWidth: 280,
    render: (row) => renderPlatformCell(row, "loon"),
  },
  {
    title: "小火箭（Shadowrocket）",
    key: "shadowrocket",
    minWidth: 280,
    render: (row) => renderPlatformCell(row, "shadowrocket"),
  },
  {
    title: "操作",
    key: "actions",
    width: 196,
    render: (row) => {
      if (!canWrite()) return "-";
      const enabled = isCustomerEnabled(row);
      const isToggling = Boolean(togglingRow[row.id]);
      const isDeleting = Boolean(deletingRow[row.id]);

      const toggleBtn = enabled
        ? h(
            NPopconfirm,
            { onPositiveClick: () => toggleCustomerStatus(row, false) },
            {
              trigger: () =>
                h(
                  NButton,
                  { size: "tiny", type: "warning", secondary: true, loading: isToggling },
                  { default: () => "暂停" },
                ),
              default: () => `确定暂停客户「${row.label}」吗？停权期间服务端将拒绝下发核心脚本。`,
            },
          )
        : h(
            NButton,
            {
              size: "tiny",
              type: "primary",
              loading: isToggling,
              onClick: () => toggleCustomerStatus(row, true),
            },
            { default: () => "恢复" },
          );

      const editBtn = h(
        NButton,
        { size: "tiny", secondary: true, onClick: () => openEditModal(row) },
        { default: () => "改备注" },
      );

      const deleteBtn = h(
        NPopconfirm,
        { onPositiveClick: () => handleDeleteCustomer(row) },
        {
          trigger: () =>
            h(
              NButton,
              { size: "tiny", type: "error", quaternary: true, loading: isDeleting },
              { default: () => "删除" },
            ),
          default: () => `确定彻底删除客户「${row.label}」及其已绑定设备凭证吗？此操作不可撤销。`,
        },
      );

      return h("div", { class: "flex flex-wrap items-center gap-4px" }, [toggleBtn, editBtn, deleteBtn]);
    },
  },
]);

// ── 授权与激活审计日志抽屉 ──
const showAuditDrawer = ref(false);
const auditEventMeta: Record<string, { label: string; type: "default" | "success" | "info" | "warning" | "error" }> = {
  customer_created: { label: "新增客户", type: "info" },
  customer_renamed: { label: "修改备注", type: "default" },
  customer_deleted: { label: "删除客户", type: "error" },
  customer_paused: { label: "暂停客户", type: "warning" },
  customer_resumed: { label: "恢复客户", type: "success" },
  global_paused: { label: "全局暂停", type: "error" },
  global_resumed: { label: "全局恢复", type: "success" },
  code_issued: { label: "签发激活码", type: "info" },
  activated: { label: "设备激活成功", type: "success" },
};

const auditColumns: DataTableColumns<CCMTVAuditItem> = [
  {
    title: "时间",
    key: "at",
    width: 145,
    render: (row) => formatDateTime(row.at, true),
  },
  {
    title: "事件",
    key: "event",
    width: 118,
    render: (row) => {
      const meta = auditEventMeta[row.event] || { label: row.event, type: "default" as const };
      return h(NTag, { size: "small", type: meta.type, bordered: false }, { default: () => meta.label });
    },
  },
  {
    title: "客户",
    key: "customer",
    minWidth: 140,
    ellipsis: { tooltip: true },
    render: (row) => row.customer_label || (row.customer_id ? `已删除 (${row.customer_id.slice(0, 8)})` : "全局配置"),
  },
  {
    title: "客户端",
    key: "platform",
    width: 90,
    render: (row) => (row.platform ? platformName(row.platform) : "-"),
  },
];

async function copyText(value: string, label: string) {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(value);
    } else {
      const textarea = document.createElement("textarea");
      textarea.value = value;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.focus();
      textarea.select();
      const copied = document.execCommand("copy");
      document.body.removeChild(textarea);
      if (!copied) throw new Error("copy failed");
    }
    window.$message?.success(`${label}已复制到剪贴板`);
  } catch {
    window.$message?.error("复制失败，请手动选中复制");
  }
}

function downloadConfigFile(cfg: { filename: string; text: string }) {
  const blob = new Blob([cfg.text], { type: "text/plain;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = cfg.filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

onMounted(load);
</script>

<template>
  <div class="flex flex-col gap-12px">
    <!-- 全局硬授权控制栏 -->
    <div class="flex flex-wrap items-center justify-between gap-12px rounded-8px border border-gray-200/70 bg-gray-50/70 p-14px dark:border-gray-700/70 dark:bg-gray-800/40">
      <div class="flex flex-col gap-4px">
        <div class="flex flex-wrap items-center gap-8px">
          <span class="text-15px font-600">全局授权（硬授权核心下发）</span>
          <NTag size="small" :type="globalEnabled ? 'success' : 'warning'" :bordered="false">
            {{ globalEnabled ? "运行中" : "已暂停" }}
          </NTag>
          <span v-if="publicOrigin" class="text-12px text-gray-400">受控域名：{{ publicOrigin }}</span>
        </div>
        <div class="text-12px text-gray-500 dark:text-gray-400">
          恢复全局后，单独暂停的客户仍保持暂停；停权期间服务端拒绝下发核心执行脚本。
        </div>
      </div>

      <NSpace v-if="canWrite()" align="center">
        <NPopconfirm v-if="globalEnabled" @positive-click="toggleGlobal(false)">
          <template #trigger>
            <NButton size="small" type="error" :loading="globalSaving">暂停全部</NButton>
          </template>
          确定暂停 CCMTV 全局核心脚本下发吗？暂停期间所有客户将立即受限。
        </NPopconfirm>
        <NPopconfirm v-else @positive-click="toggleGlobal(true)">
          <template #trigger>
            <NButton size="small" type="primary" :loading="globalSaving">恢复全部</NButton>
          </template>
          确定恢复 CCMTV 全局授权下发吗？（已单独暂停的客户仍会保持暂停状态）
        </NPopconfirm>
      </NSpace>
    </div>

    <!-- 客户列表筛选与操作工具栏 -->
    <div class="flex flex-wrap items-center justify-between gap-8px">
      <div class="flex flex-wrap items-center gap-10px">
        <FilterTabs v-model:value="statusFilter" :options="statusTabs" :counts="statusCounts" />
        <span class="text-12px text-gray-400">共 {{ customers.length }} 位客户</span>
      </div>
      <div class="flex flex-wrap items-center gap-8px">
        <NInput
          v-model:value="keyword"
          size="small"
          clearable
          placeholder="搜索客户备注 / 激活码 / ID"
          class="w-220px"
        />
        <NButton size="small" @click="showAuditDrawer = true">授权日志 ({{ audits.length }})</NButton>
        <NButton size="small" :loading="loading" @click="load">刷新</NButton>
        <NButton v-if="canWrite()" size="small" type="primary" @click="openAddModal">添加客户</NButton>
      </div>
    </div>

    <!-- 客户授权状态表格 -->
    <NDataTable
      :columns="columns"
      :data="filteredCustomers"
      :loading="loading"
      size="small"
      :row-key="(r: CCMTVCustomer) => r.id"
      :max-height="560"
      :scroll-x="960"
    />

    <!-- 添加客户弹窗 -->
    <NModal
      v-model:show="showAddModal"
      preset="card"
      title="添加客户"
      :closable="!adding"
      :mask-closable="!adding"
      :close-on-esc="!adding"
      style="width: 440px; max-width: 96vw"
    >
      <NForm label-placement="top" @submit.prevent="submitAddCustomer">
        <NFormItem label="客户备注" required>
          <NInput
            v-model:value="newLabel"
            maxlength="80"
            show-count
            placeholder="输入便于识别的称呼（如：张三 / 订单号）"
            @keyup.enter="submitAddCustomer"
          />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="flex justify-end gap-8px">
          <NButton size="small" :disabled="adding" @click="showAddModal = false">取消</NButton>
          <NButton size="small" type="primary" :loading="adding" @click="submitAddCustomer">确认添加</NButton>
        </div>
      </template>
    </NModal>

    <!-- 修改客户备注弹窗 -->
    <NModal
      v-model:show="showEditModal"
      preset="card"
      title="修改客户备注"
      :closable="!editing"
      :mask-closable="!editing"
      :close-on-esc="!editing"
      style="width: 440px; max-width: 96vw"
    >
      <NForm label-placement="top" @submit.prevent="submitEditCustomer">
        <NFormItem label="客户备注" required>
          <NInput
            v-model:value="editLabel"
            maxlength="80"
            show-count
            placeholder="输入便于识别的称呼（如：张三 / 订单号）"
            @keyup.enter="submitEditCustomer"
          />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="flex justify-end gap-8px">
          <NButton size="small" :disabled="editing" @click="showEditModal = false">取消</NButton>
          <NButton size="small" type="primary" :loading="editing" @click="submitEditCustomer">保存</NButton>
        </div>
      </template>
    </NModal>

    <!-- 激活码查看 / 签发弹窗 -->
    <NModal
      v-model:show="showIssuedModal"
      preset="card"
      :title="issuedInfo?.reused ? '激活凭证与配置详情' : '激活码已生成'"
      style="width: 580px; max-width: 96vw"
    >
      <div v-if="issuedInfo" class="flex flex-col gap-12px">
        <div class="text-13px text-gray-600 dark:text-gray-300">
          <span class="font-600">{{ issuedInfo.customer_label }}</span>
          · {{ platformName(issuedInfo.platform) }} · 一码一机 · 首次激活后自动绑定设备
        </div>

        <div>
          <div class="mb-4px text-12px text-gray-500">激活码</div>
          <div class="flex items-center gap-8px">
            <NInput :value="issuedInfo.code" readonly class="font-mono" />
            <NButton size="small" @click="copyText(issuedInfo.code, '激活码')">复制</NButton>
          </div>
        </div>

        <div>
          <div class="mb-4px text-12px text-gray-500">正式配置导入链接（直接发给客户）</div>
          <div class="flex items-center gap-8px">
            <NInput :value="issuedInfo.release_url" readonly class="font-mono" />
            <NButton size="small" @click="copyText(issuedInfo.release_url, '正式导入链接')">复制</NButton>
          </div>
          <div class="mt-8px">
            <NButton size="small" type="primary" @click="downloadConfigFile(issuedInfo.release_config)">
              下载正式配置 ({{ issuedInfo.release_config.filename }})
            </NButton>
          </div>
        </div>

        <NCollapse>
          <NCollapseItem title="验收工具：只读测试配置（不执行考勤）" name="probe">
            <div class="flex flex-col gap-8px">
              <div class="text-12px text-gray-500">只读测试导入链接</div>
              <div class="flex items-center gap-8px">
                <NInput :value="issuedInfo.probe_url" readonly class="font-mono" />
                <NButton size="small" @click="copyText(issuedInfo.probe_url, '测试导入链接')">复制</NButton>
              </div>
              <div>
                <NButton size="small" @click="downloadConfigFile(issuedInfo.probe_config)">
                  下载只读测试配置 ({{ issuedInfo.probe_config.filename }})
                </NButton>
              </div>
            </div>
          </NCollapseItem>
        </NCollapse>

        <NAlert type="info" :bordered="false" :show-icon="false">
          该激活码采用「一码一机」一次性绑定机制：在设备成功激活前可随时在后台回看链接；一旦首台设备激活成功，激活码立即作废以防群发白嫖。
        </NAlert>
      </div>
    </NModal>

    <!-- 授权与激活审计日志抽屉 -->
    <NDrawer v-model:show="showAuditDrawer" width="min(720px, 96vw)">
      <NDrawerContent title="CCMTV 授权与激活日志（最近 100 条）" closable>
        <div class="mb-10px flex items-center justify-between text-12px text-gray-500">
          <span>包含创建客户、签发激活码、设备激活、启停变更及删除记录。</span>
          <NButton size="tiny" :loading="loading" @click="load">刷新</NButton>
        </div>
        <NDataTable
          :columns="auditColumns"
          :data="audits"
          size="small"
          :row-key="(r: CCMTVAuditItem) => r.id"
          :max-height="620"
        />
      </NDrawerContent>
    </NDrawer>
  </div>
</template>
