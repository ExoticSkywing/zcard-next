package inhouse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/NovaWorks/zcard-next/server/internal/mods/authz"
	"github.com/go-kratos/kratos/v3/errors"
)

func TestCCMTVPermissionsDeclared(t *testing.T) {
	dir := authz.BuildDirectory()
	routes := []authz.RouteInfo{
		{Method: http.MethodGet, Path: "/api/v1/admin/inhouse/ccmtv/state"},
		{Method: http.MethodPost, Path: "/api/v1/admin/inhouse/ccmtv/global"},
		{Method: http.MethodPost, Path: "/api/v1/admin/inhouse/ccmtv/customers"},
		{Method: http.MethodPost, Path: "/api/v1/admin/inhouse/ccmtv/customer-update"},
		{Method: http.MethodPost, Path: "/api/v1/admin/inhouse/ccmtv/customer-delete"},
		{Method: http.MethodPost, Path: "/api/v1/admin/inhouse/ccmtv/customer-status"},
		{Method: http.MethodPost, Path: "/api/v1/admin/inhouse/ccmtv/issue"},
	}
	if err := dir.Reconcile(routes, "/api/v1/admin/"); err != nil {
		t.Fatalf("inhouse routes reconcile failed: %v", err)
	}
}

func TestResolveCCMTVKeyFromEnvFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(tmp, []byte("PUBLIC_ORIGIN=https://ccm.1yo.cc\nADMIN_PASSWORD='secret-password-123456'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZCARD_CCMTV_KEY", "")
	t.Setenv("ZCARD_CCMTV_ENV_FILE", tmp)
	if got := resolveCCMTVKey(); got != "secret-password-123456" {
		t.Fatalf("resolveCCMTVKey() = %q, want %q", got, "secret-password-123456")
	}
}

func TestCallCCMTVTranslatesUpstream401To503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-inhouse-key") != "valid-key" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 401, "message": "内网管控密钥无效"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": true})
	}))
	defer srv.Close()

	t.Setenv("ZCARD_CCMTV_URL", srv.URL)
	t.Setenv("ZCARD_CCMTV_KEY", "wrong-key")

	_, err := callCCMTV(context.Background(), http.MethodGet, "/api/inhouse/state", nil)
	if !errors.IsServiceUnavailable(err) {
		t.Fatalf("expected 503 ServiceUnavailable instead of 401 to avoid admin logout, got: %v", err)
	}

	t.Setenv("ZCARD_CCMTV_KEY", "valid-key")
	res, err := callCCMTV(context.Background(), http.MethodGet, "/api/inhouse/state", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok || m["enabled"] != true {
		t.Fatalf("unexpected response payload: %#v", res)
	}
}
