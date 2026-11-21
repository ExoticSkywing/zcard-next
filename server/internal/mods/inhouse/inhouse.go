package inhouse

// 自研产品（In-house Products）统一管理后台网关模块：
// - 通过 authz.Declare 自注册权限点，与 authz/permissions.go 零文件冲突；
// - 通过 srv.Route 注册并透传 Kratos Operation，天然复用管理员 JWT、RBAC 鉴权与操作审计日志；
// - 未来新增自研产品只需在本包内新增 <product>.go 并在此处调用 registerXxxRoutes(srv)。

import (
	"context"
	"net/http"

	khttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/types/known/emptypb"
)

func init() {
	registerCCMTVPermissions()
}

// Register 向 HTTP Server 挂载全部自研产品管理路由（必须在 reconcileRoutes 之前调用）。
func Register(srv *khttp.Server) {
	registerCCMTVRoutes(srv)
}

// registerAdminJSONRoute 参照 media/video.go 原生写法，将自定义 HTTP 路由接入 Kratos 管理端中间件链与审计对账。
func registerAdminJSONRoute(
	srv *khttp.Server,
	method string,
	path string,
	op string,
	fn func(ctx context.Context, r *http.Request) (any, error),
) {
	handle := func(c khttp.Context) error {
		khttp.SetOperation(c, "/"+op)
		h := c.Middleware(func(ctx context.Context, _ any) (any, error) {
			return fn(ctx, c.Request())
		})
		out, err := h(c, &emptypb.Empty{})
		if err != nil {
			return err
		}
		return c.Result(http.StatusOK, out)
	}

	r := srv.Route("/")
	switch method {
	case http.MethodGet:
		r.GET(path, handle)
	case http.MethodPost:
		r.POST(path, handle)
	case http.MethodPut:
		r.PUT(path, handle)
	case http.MethodDelete:
		r.DELETE(path, handle)
	default:
		panic("inhouse: unsupported HTTP method: " + method)
	}
}
