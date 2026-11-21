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
	"github.com/NovaWorks/zcard-next/server/internal/mods/supply/adapter"
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

func TestCCMTVSupplyAdapterHandshake(t *testing.T) {
	if os.Getenv("ZCARD_CCMTV_SUPPLY_TEST") != "1" {
		t.Skip("set ZCARD_CCMTV_SUPPLY_TEST=1 and ZCARD_HTTPX_ALLOW_PRIVATE=1 to verify live ccmtv supply endpoint")
	}
	key := resolveCCMTVKey()
	if key == "" {
		t.Fatal("resolveCCMTVKey() returned empty")
	}
	a, err := adapter.New("zcard", resolveCCMTVBaseURL(), adapter.Credentials{
		APIKey:    "ccmtv",
		APISecret: key,
	}, []int{0})
	if err != nil {
		t.Fatalf("adapter.New failed: %v", err)
	}
	ctx := context.Background()
	ping, err := a.Ping(ctx)
	if err != nil || ping.ProtocolVersion != "zcard-supply-v2" || ping.Currency != "CNY" {
		t.Fatalf("Ping failed: %+v, err=%v", ping, err)
	}
	cats, err := a.ListCategories(ctx)
	if err != nil || len(cats) == 0 || cats[0].ID != "ccmtv" {
		t.Fatalf("ListCategories failed: %+v, err=%v", cats, err)
	}
	list, err := a.ListProducts(ctx, 1, 50, true)
	if err != nil || list.Total != 2 || len(list.Items) != 2 {
		t.Fatalf("ListProducts failed: %+v, err=%v", list, err)
	}
	stock, err := a.GetStock(ctx, "ccmtv-loon", "")
	if err != nil || stock <= 0 {
		t.Fatalf("GetStock failed: stock=%d, err=%v", stock, err)
	}
}
