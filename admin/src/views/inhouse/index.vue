<script setup lang="ts">
// 自研产品管理容器（In-house Products）：各独立自研服务的统一管理入口。
// 权限：inhouse:read（查看）、inhouse:write（开关/新增客户/签发凭证）。
// 支持 ?tab= 深链直达具体自研产品标签页。
import { defineAsyncComponent, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { NCard, NTabPane, NTabs } from "naive-ui";
import { checkAuth } from "@/directives";
import CCMTVTab from "./components/ccmtv-tab.vue";

// 后续新增第 N 个自研业务时，在此按懒加载方式追加 Tab 组件即可：
// const FutureProductTab = defineAsyncComponent(() => import("./components/future-tab.vue"));
void defineAsyncComponent;

defineOptions({ name: "InhouseProducts" });

const route = useRoute();
const visibleTabs = [
  { key: "ccmtv", auth: "inhouse:read" },
].filter((t) => checkAuth(t.auth));

const activeTab = ref<string>(
  visibleTabs.some((t) => t.key === route.query.tab) ? String(route.query.tab) : (visibleTabs[0]?.key || "ccmtv"),
);
watch(
  () => route.query.tab,
  (tab) => {
    if (visibleTabs.some((t) => t.key === tab)) activeTab.value = String(tab);
  },
);
</script>

<template>
  <div class="min-h-500px">
    <NCard title="自研产品">
      <NTabs v-model:value="activeTab" type="line">
        <NTabPane v-if="checkAuth('inhouse:read')" name="ccmtv" tab="CCMTV 授权管理">
          <CCMTVTab />
        </NTabPane>
      </NTabs>
    </NCard>
  </div>
</template>
