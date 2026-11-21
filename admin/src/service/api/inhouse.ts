// 自研产品（In-house Products）后台管控 API（inhouse:read / inhouse:write）。

import { request } from "../request";

export type CCMTVPlatform = "loon" | "shadowrocket";

export interface CCMTVInstallSlot {
  customer_id: string;
  platform: CCMTVPlatform;
  activated_at?: number | null;
  expires_at?: number | null;
  last_seen_at?: number | null;
  code_plain?: string | null;
  pending?: number | boolean | null;
  can_view_code?: number | boolean | null;
}

export interface CCMTVCustomer {
  id: string;
  label: string;
  enabled: number | boolean;
  created_at?: number | null;
  installs: Partial<Record<CCMTVPlatform, CCMTVInstallSlot>>;
}

export interface CCMTVAuditItem {
  id: number;
  event: string;
  customer_id?: string | null;
  customer_label?: string | null;
  platform?: CCMTVPlatform | null;
  at: number;
}

export interface CCMTVState {
  enabled: boolean;
  public_origin: string;
  customers: CCMTVCustomer[];
  audits?: CCMTVAuditItem[];
  id?: string;
}

export interface CCMTVIssueResult {
  ok: boolean;
  reused?: boolean;
  customer_id: string;
  customer_label: string;
  platform: CCMTVPlatform;
  code: string;
  release_url: string;
  probe_url: string;
  release_config: { filename: string; text: string };
  probe_config: { filename: string; text: string };
}

export function fetchCCMTVState() {
  return request<CCMTVState>({ url: "/api/v1/admin/inhouse/ccmtv/state" });
}

export function setCCMTVGlobal(enabled: boolean) {
  return request<CCMTVState>({
    url: "/api/v1/admin/inhouse/ccmtv/global",
    method: "post",
    data: { enabled },
  });
}

export function createCCMTVCustomer(label: string) {
  return request<CCMTVState>({
    url: "/api/v1/admin/inhouse/ccmtv/customers",
    method: "post",
    data: { label },
  });
}

export function updateCCMTVCustomer(id: string, label: string) {
  return request<CCMTVState>({
    url: "/api/v1/admin/inhouse/ccmtv/customer-update",
    method: "post",
    data: { id, label },
  });
}

export function deleteCCMTVCustomer(id: string) {
  return request<CCMTVState>({
    url: "/api/v1/admin/inhouse/ccmtv/customer-delete",
    method: "post",
    data: { id },
  });
}

export function setCCMTVCustomerStatus(id: string, enabled: boolean) {
  return request<CCMTVState>({
    url: "/api/v1/admin/inhouse/ccmtv/customer-status",
    method: "post",
    data: { id, enabled },
  });
}

export function issueCCMTVCode(id: string, platform: CCMTVPlatform, reuse = false) {
  return request<CCMTVIssueResult>({
    url: "/api/v1/admin/inhouse/ccmtv/issue",
    method: "post",
    data: { id, platform, reuse },
  });
}
