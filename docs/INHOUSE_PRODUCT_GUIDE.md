# ZCard「自研产品（In-house Products）」低成本微创扩展开发规范

> 本文档定义在 `zcard-next` 中持续接入第 `1 ~ N` 个自研服务（如 CCMTV 授权中心、后续医学科研/效率工具等）的官方标准范式，目标是在 **100% 复用 ZCard 原生鉴权/权限/审计/UI 体系** 的同时，确保 **高频拉取上游更新（`git pull --rebase`）几乎零冲突**。

---

## 1. 核心架构分层与命名规范

1. **统一命名与菜单归属**
   - 后台一级导航名称：**`自研产品`**（英文：`In-house Products`）
   - 路由与模块标识：**`inhouse`**（访问路径 `/inhouse`，菜单排序 `order: 10.5`，位于「渠道管理」之后、「系统设置」之前）
   - 后端 API 统一前缀：`/api/v1/admin/inhouse/<product>/*`
   - RBAC 权限域：`inhouse`（`inhouse:read` 查看控制台、`inhouse:write` 执行授权/启停等写操作）
   - **扩展铁律**：无论未来新增多少个自研业务，左侧一级导航永远只有 **`自研产品`** 这 1 个入口，具体产品以 **顶部横向标签页（`<NTabPane>`）** 形式在页面内扩展，支持 `?tab=<product>` 深链。

2. **自研服务的双切面标准契约**
   所有独立部署的自研服务（Node.js / Python / Go，建议仅监听 `127.0.0.1:<port>`），只需暴露纯 API，不再各自编写管理员 HTML 登录页与 Cookie/CSRF：
   - **板块一：自动售卖切面（`/api/supply/*`）**：对接 ZCard 原生「渠道管理 -> 货源渠道」，用于自动上报 SKU 与实时下单发卡/发码。
   - **板块二：后台管控切面（`/api/inhouse/*`）**：通过 `x-inhouse-key` 内网密钥鉴权，提供纯 JSON 管控接口（如总开关、客户列表、单客启停、凭证签发/重置），由 ZCard 后端代理转发给 ZCard 管理后台视图。

---

## 2. 上游同步零冲突原则（白名单修改点）

除以下极少数公共挂载点外，**所有新增代码必须 100% 收敛在 `inhouse` 独立目录中**，严禁修改上游 `.proto`、Wire 依赖注入文件（`wire.go`）或既有业务模块目录：

| 层次 | 独立新增目录 / 文件（自由扩展，永不冲突） | 对上游既有文件的微创修改（仅追加 1 行，后续几乎无需再改） |
| :--- | :--- | :--- |
| **后端 Go** | `server/internal/mods/inhouse/` | 1. `server/internal/server/http.go`：import 包并追加 `inhouse.Register(srv)`<br>2. `server/internal/mods/authz/directory.go`：`domainLabels` 中追加 `"inhouse": "自研产品",` |
| **前端 Vue** | `admin/src/views/inhouse/`<br>`admin/src/service/api/inhouse.ts` | 1. `admin/src/service/api/index.ts`：追加 `export * from "./inhouse";`<br>2. `admin/src/router/permissions.ts`：追加 `inhouse: ["inhouse:read"],`<br>3. `admin/src/locales/langs/{zh-cn,en-us}.ts`：`route` 下追加 `inhouse`<br>4. 路由生成物（`routes.ts` / `imports.ts` / `transform.ts` / `elegant-router.d.ts`）由构建工具自动维护 |

---

## 3. 新增第 N 个自研产品的标准 4 步 SOP（以 `<product>` 为例）

### 第 1 步：自研服务侧暴露 `/api/inhouse/*` 纯 JSON 接口
- 必须校验内网请求头（如 `x-inhouse-key`）的合法性；
- 所有异常响应建议统一返回 HTTP 状态码 + JSON `{ "code": 400, "message": "中文可读错误提示" }`。

### 第 2 步：在 ZCard 后端新增适配器 `server/internal/mods/inhouse/<product>.go`
参照 1 号样板 [ccmtv.go](file:///www/wwwroot/xingxy.manyuzo.com/wp-content/plugins/services/zcard-next/server/internal/mods/inhouse/ccmtv.go)：
1. 定义权限声明函数 `register<Product>Permissions()`，通过 `authz.Declare(...)` 声明每条路由的 `Code`（`inhouse:read` 或 `inhouse:write`）、`Domain: "inhouse"`、`Op`、`Method`、`Path`，并在 [inhouse.go](file:///www/wwwroot/xingxy.manyuzo.com/wp-content/plugins/services/zcard-next/server/internal/mods/inhouse/inhouse.go) 的 `init()` 中调用（确保启动时 `reconcileRoutes` 自动对账通过，并纳入 `OpAuditFilter` 审计）；
2. 定义路由注册函数 `register<Product>Routes(srv *khttp.Server)`，复用 `registerAdminJSONRoute(...)` 挂载 `/api/v1/admin/inhouse/<product>/*`，并在 `inhouse.go` 的 `Register(srv)` 中调用；
3. **安全红线**：如果上游内网服务返回 `401/403`，适配器必须转换为 `503 ServiceUnavailable`（例如 `errors.ServiceUnavailable(...)`），**绝不可原样透传 HTTP `401` 给前端**，否则前端拦截器会误以为 ZCard 管理员登录态失效并强制登出！

### 第 3 步：在 ZCard 前端新增 API 与标签页
1. 在 `admin/src/service/api/inhouse.ts` 中新增 `<Product>` 的 TypeScript 接口与请求函数；
2. 在 `admin/src/views/inhouse/components/<product>-tab.vue` 中编写业务面板（使用 Naive UI 组件、`FilterTabs`、`checkAuth('inhouse:write')` 控制写操作按钮可见性）；
3. 在 `admin/src/views/inhouse/index.vue` 的 `visibleTabs` 与 `<NTabs>` 中引入 `<ProductTab />`（推荐用 `defineAsyncComponent` 按需懒加载，并配 `display-directive="show:lazy"`）。

### 第 4 步：安全编译与重启验证（防本机 OOM 铁律）
1. **前端构建**（务必限制 Node 堆内存，构建后同步到嵌入目录）：
   ```bash
   cd admin && NODE_OPTIONS="--max-old-space-size=1536" ./node_modules/.bin/vite build --mode prod
   rm -rf ../server/web/admin/* && cp -r dist/* ../server/web/admin/
   ```
2. **后端全栈编译**（务必加 `-p 1` 限制单包并发编译，绝不传过小 `GOMEMLIMIT`）：
   ```bash
   cd server && GOTOOLCHAIN=auto go build -p 1 -tags fullstack -o bin/zcard ./cmd/zcard
   ```
3. **重启与校验**：
   ```bash
   systemctl restart zcard
   journalctl -u zcard -n 30 --no-pager
   ```
   确认日志出现 `authz.reconciled` 且 `curl http://127.0.0.1:8000/health` 正常。

---

## 4. 双分支隔离与上游同步更新（Rebase / Merge）操作手册

### 4.1 远程源与分支规范（严格隔离，绝不污染 `main`）
- **`upstream` 远程源**（`https://github.com/NovaWorks/zcard-next.git`）：官方上游源仓库，**只读拉取**。
- **`origin` 远程源**（`git@github.com:ExoticSkywing/zcard-next.git`）：个人 Fork 仓库，用于同步官方纯净 `main` 及备份二改分支 `custom`。
- **`main` 分支**：永远保持与官方上游 `upstream/main` 100% 一致的纯净状态，**严禁直接在 `main` 上提交二改代码**。
- **`custom` 分支**：当前生产与二改主分支，所有 `inhouse` 自研产品扩展与定制功能统一在 `custom` 分支开发和提交。

### 4.2 同步上游官方更新标准流程
当官方 `NovaWorks/zcard-next` 发布新版本需要吸收更新时：
1. 拉取官方 `upstream/main` 并快进更新本地纯净 `main`：
   ```bash
   git fetch upstream main
   git checkout main && git merge --ff-only upstream/main
   ```
2. 切回 `custom` 分支执行变基重放（把你的二改干净叠到最新版之上）：
   ```bash
   git checkout custom
   git rebase main
   ```
3. 推送更新后的 `main` 与 `custom` 到个人 Fork 仓库 (`origin`) 备份：
   ```bash
   git push origin main
   git push origin custom --force-with-lease
   ```
4. 极少数的自动生成路由冲突处理：
   - 若上游改动了 `admin/src/router/elegant/*` 或 `routes.ts` 产生冲突：直接接受上游版本，随后在 `admin/` 目录执行 `./node_modules/.bin/sa gen-route`（或重新 `vite build`），插件会自动把 `src/views/inhouse/index.vue` 重新合并进路由生成物，**无需手动解路由冲突**；
   - 核对 `server/internal/server/http.go` 与 `server/internal/mods/authz/directory.go` 的单行挂载点是否仍在即可。
