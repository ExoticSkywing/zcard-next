package inhouse

// CCMTV 授权中心管控适配器：转发 ZCard 后台请求至本机 ccmtv-license (/api/inhouse/*)。

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/NovaWorks/zcard-next/server/internal/mods/authz"
	"github.com/go-kratos/kratos/v3/errors"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

const (
	opCCMTVState          = "zcard.api.admin.v1.AdminInhouseService/GetCCMTVState"
	opCCMTVGlobal         = "zcard.api.admin.v1.AdminInhouseService/SetCCMTVGlobal"
	opCCMTVCustomerCreate = "zcard.api.admin.v1.AdminInhouseService/CreateCCMTVCustomer"
	opCCMTVCustomerUpdate = "zcard.api.admin.v1.AdminInhouseService/UpdateCCMTVCustomer"
	opCCMTVCustomerDelete = "zcard.api.admin.v1.AdminInhouseService/DeleteCCMTVCustomer"
	opCCMTVCustomerStatus = "zcard.api.admin.v1.AdminInhouseService/SetCCMTVCustomerStatus"
	opCCMTVIssue          = "zcard.api.admin.v1.AdminInhouseService/IssueCCMTVCode"

	defaultCCMTVURL     = "http://127.0.0.1:29417"
	defaultCCMTVEnvFile = "/root/data/repo/ccmtv-license/.env"
	maxInhouseBodyBytes = 64 * 1024
)

var ccmtvHTTPClient = &http.Client{Timeout: 8 * time.Second}

func registerCCMTVPermissions() {
	authz.Declare(
		authz.Perm{
			Code: "inhouse:read", Desc: "查看自研产品控制台", Domain: "inhouse",
			Op: opCCMTVState, Method: http.MethodGet, Path: "/api/v1/admin/inhouse/ccmtv/state",
		},
		authz.Perm{
			Code: "inhouse:write", Desc: "管理自研产品授权与开关", Domain: "inhouse", AdminOnly: true,
			Op: opCCMTVGlobal, Method: http.MethodPost, Path: "/api/v1/admin/inhouse/ccmtv/global",
		},
		authz.Perm{
			Code: "inhouse:write", Desc: "管理自研产品授权与开关", Domain: "inhouse", AdminOnly: true,
			Op: opCCMTVCustomerCreate, Method: http.MethodPost, Path: "/api/v1/admin/inhouse/ccmtv/customers",
		},
		authz.Perm{
			Code: "inhouse:write", Desc: "管理自研产品授权与开关", Domain: "inhouse", AdminOnly: true,
			Op: opCCMTVCustomerUpdate, Method: http.MethodPost, Path: "/api/v1/admin/inhouse/ccmtv/customer-update",
		},
		authz.Perm{
			Code: "inhouse:write", Desc: "管理自研产品授权与开关", Domain: "inhouse", AdminOnly: true,
			Op: opCCMTVCustomerDelete, Method: http.MethodPost, Path: "/api/v1/admin/inhouse/ccmtv/customer-delete",
		},
		authz.Perm{
			Code: "inhouse:write", Desc: "管理自研产品授权与开关", Domain: "inhouse", AdminOnly: true,
			Op: opCCMTVCustomerStatus, Method: http.MethodPost, Path: "/api/v1/admin/inhouse/ccmtv/customer-status",
		},
		authz.Perm{
			Code: "inhouse:write", Desc: "管理自研产品授权与开关", Domain: "inhouse", AdminOnly: true,
			Op: opCCMTVIssue, Method: http.MethodPost, Path: "/api/v1/admin/inhouse/ccmtv/issue",
		},
	)
}

func registerCCMTVRoutes(srv *khttp.Server) {
	registerAdminJSONRoute(srv, http.MethodGet, "/api/v1/admin/inhouse/ccmtv/state", opCCMTVState,
		func(ctx context.Context, _ *http.Request) (any, error) {
			return callCCMTV(ctx, http.MethodGet, "/api/inhouse/state", nil)
		})
	registerAdminJSONRoute(srv, http.MethodPost, "/api/v1/admin/inhouse/ccmtv/global", opCCMTVGlobal,
		func(ctx context.Context, r *http.Request) (any, error) {
			return forwardCCMTVPost(ctx, r, "/api/inhouse/global")
		})
	registerAdminJSONRoute(srv, http.MethodPost, "/api/v1/admin/inhouse/ccmtv/customers", opCCMTVCustomerCreate,
		func(ctx context.Context, r *http.Request) (any, error) {
			return forwardCCMTVPost(ctx, r, "/api/inhouse/customers")
		})
	registerAdminJSONRoute(srv, http.MethodPost, "/api/v1/admin/inhouse/ccmtv/customer-update", opCCMTVCustomerUpdate,
		func(ctx context.Context, r *http.Request) (any, error) {
			return forwardCCMTVPost(ctx, r, "/api/inhouse/customer-update")
		})
	registerAdminJSONRoute(srv, http.MethodPost, "/api/v1/admin/inhouse/ccmtv/customer-delete", opCCMTVCustomerDelete,
		func(ctx context.Context, r *http.Request) (any, error) {
			return forwardCCMTVPost(ctx, r, "/api/inhouse/customer-delete")
		})
	registerAdminJSONRoute(srv, http.MethodPost, "/api/v1/admin/inhouse/ccmtv/customer-status", opCCMTVCustomerStatus,
		func(ctx context.Context, r *http.Request) (any, error) {
			return forwardCCMTVPost(ctx, r, "/api/inhouse/customer-status")
		})
	registerAdminJSONRoute(srv, http.MethodPost, "/api/v1/admin/inhouse/ccmtv/issue", opCCMTVIssue,
		func(ctx context.Context, r *http.Request) (any, error) {
			return forwardCCMTVPost(ctx, r, "/api/inhouse/issue")
		})
}

func forwardCCMTVPost(ctx context.Context, r *http.Request, upstreamPath string) (any, error) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxInhouseBodyBytes+1))
	if err != nil {
		return nil, errors.BadRequest("inhouse.READ_BODY_FAILED", "读取请求内容失败")
	}
	if len(raw) > maxInhouseBodyBytes {
		return nil, errors.BadRequest("inhouse.BODY_TOO_LARGE", "请求体过大")
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte("{}")
	}
	return callCCMTV(ctx, http.MethodPost, upstreamPath, raw)
}

func callCCMTV(ctx context.Context, method, path string, body []byte) (any, error) {
	key := resolveCCMTVKey()
	if key == "" {
		return nil, errors.ServiceUnavailable("inhouse.CCMTV_KEY_MISSING", "未配置 CCMTV 内网管控密钥（ZCARD_CCMTV_KEY）")
	}

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, resolveCCMTVBaseURL()+path, reqBody)
	if err != nil {
		return nil, errors.InternalServer("inhouse.REQUEST_BUILD_FAILED", "构建 CCMTV 请求失败")
	}
	req.Header.Set("x-inhouse-key", key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := ccmtvHTTPClient.Do(req)
	if err != nil {
		return nil, errors.ServiceUnavailable("inhouse.CCMTV_UNREACHABLE", "CCMTV 授权服务暂不可达，请检查服务是否运行")
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, errors.ServiceUnavailable("inhouse.CCMTV_READ_FAILED", "读取 CCMTV 响应失败")
	}

	var payload map[string]any
	if len(bytes.TrimSpace(respBytes)) > 0 {
		if jsonErr := json.Unmarshal(respBytes, &payload); jsonErr != nil {
			return nil, errors.ServiceUnavailable("inhouse.CCMTV_BAD_RESPONSE", "CCMTV 服务返回了无效格式数据")
		}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return payload, nil
	}

	msg := "CCMTV 服务处理失败"
	if m, ok := payload["message"].(string); ok && m != "" {
		msg = m
	}
	switch resp.StatusCode {
	case http.StatusBadRequest:
		return nil, errors.BadRequest("inhouse.CCMTV_BAD_REQUEST", msg)
	case http.StatusNotFound:
		return nil, errors.NotFound("inhouse.CCMTV_NOT_FOUND", msg)
	case http.StatusUnauthorized, http.StatusForbidden:
		// 切勿向上游透传 401，否则前端请求拦截器会误判 ZCard 管理员登录态失效并强制登出。
		return nil, errors.ServiceUnavailable("inhouse.CCMTV_AUTH_FAILED", "CCMTV 内网管控密钥校验失败，请核对配置")
	default:
		return nil, errors.ServiceUnavailable("inhouse.CCMTV_ERROR", msg)
	}
}

func resolveCCMTVBaseURL() string {
	base := strings.TrimSpace(os.Getenv("ZCARD_CCMTV_URL"))
	if base == "" {
		base = defaultCCMTVURL
	}
	return strings.TrimRight(base, "/")
}

func resolveCCMTVKey() string {
	if key := strings.TrimSpace(os.Getenv("ZCARD_CCMTV_KEY")); key != "" {
		return key
	}
	envPath := strings.TrimSpace(os.Getenv("ZCARD_CCMTV_ENV_FILE"))
	if envPath == "" {
		envPath = defaultCCMTVEnvFile
	}
	data, err := os.ReadFile(envPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if val, ok := strings.CutPrefix(line, "ADMIN_PASSWORD="); ok {
			val = strings.TrimSpace(val)
			if len(val) >= 2 && ((val[0] == '\'' && val[len(val)-1] == '\'') || (val[0] == '"' && val[len(val)-1] == '"')) {
				val = val[1 : len(val)-1]
			}
			return val
		}
	}
	return ""
}
