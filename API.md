# PackPilot API 对接文档

PackPilot 提供基于 JSON 的 REST API、SSE 任务事件流和文本文件下载。机器可读规范位于：

- OpenAPI 3.1：`GET /api/openapi.yaml`
- 轻量文档页：`GET /api/docs`
- 本文件包含中文接入流程和示例，可直接导入 Postman、Apifox、Insomnia 或 OpenAPI 代码生成器。

以下示例以 `http://localhost:8080` 为服务地址，生产环境请替换为 HTTPS 地址。

## 通用约定

- 请求和响应 JSON 使用 UTF-8，创建/写操作使用 `Content-Type: application/json`。
- 时间字段为 RFC 3339 字符串。
- 任务 ID 为 16 位小写十六进制字符串。
- 错误统一为 `{"error":"可读错误信息"}`。
- 除登录、认证状态和文档外，所有接口都需要 `packpilot_session` Cookie。
- Cookie 带有 HttpOnly 和 SameSite=Lax；跨域客户端必须携带凭据，网页请求使用 `credentials: "include"`，Android 使用 CookieManager。
- 写接口会校验 Origin；非浏览器客户端通常不发送 Origin，不受额外 Origin 限制。

## 认证流程

### 查询认证状态

```http
GET /api/auth/status
```

响应：

```json
{"enabled":true,"authenticated":false}
```

### 登录

```bash
curl -c cookies.txt -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"password":"你的页面密码"}'
```

成功后响应 `Set-Cookie: packpilot_session=...; HttpOnly; SameSite=Lax`。密码只用于登录，客户端不应持久化密码。

错误码：

- `400`：JSON 格式错误。
- `401`：密码错误或会话无效。
- `429`：连续失败过多，触发限流。

### 退出

```bash
curl -b cookies.txt -c cookies.txt -X POST http://localhost:8080/api/auth/logout
```

## 系统状态

```bash
curl -b cookies.txt http://localhost:8080/api/health
```

```json
{
  "status":"ok",
  "docker":"ok",
  "worker":{"concurrency":2,"running":1,"queued":3},
  "time":"2026-10-28T12:00:00Z"
}
```

## 任务接口

### 创建任务

```bash
curl -b cookies.txt -X POST http://localhost:8080/api/tasks \
  -H 'Content-Type: application/json' \
  -d '{"source_url":"https://github.com/jpmens/jo","package_name":"jo","publish":false}'
```

字段：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `source_url` | string | 是 | 不带凭据的 HTTP(S) 仓库或官网地址 |
| `package_name` | string | 否 | 包名提示，最长 128 字符 |
| `publish` | boolean | 否 | 成功后自动发布，还需服务端启用 AUR |

返回 `201` 和完整 Task。后续任务会自动经历 `queued`、`discovering`、`building`、`succeeded` 等状态。

### 列表与详情

```bash
curl -b cookies.txt http://localhost:8080/api/tasks
curl -b cookies.txt http://localhost:8080/api/tasks/0123456789abcdef
```

列表响应为 `{"tasks":[...]}`。详情包含 `logs`、`build_log`、`pkgbuild`、`srcinfo` 和可选 `publication`。

### SSE 实时事件

```bash
curl -N -b cookies.txt http://localhost:8080/api/tasks/0123456789abcdef/events
```

每 1.5 秒发送：

```text
event: task
data: {"id":"0123456789abcdef","status":"building",...}

```

任务进入 `succeeded`、`failed`、`canceled` 或 `published` 后关闭。浏览器使用 `new EventSource(url, {withCredentials:true})`；不支持 SSE 的客户端可每 2 秒轮询任务详情。

### 取消、重试、发布和删除

```bash
curl -b cookies.txt -X POST http://localhost:8080/api/tasks/0123456789abcdef/cancel
curl -b cookies.txt -X POST http://localhost:8080/api/tasks/0123456789abcdef/retry
curl -b cookies.txt -X POST http://localhost:8080/api/tasks/0123456789abcdef/publish
curl -b cookies.txt -X DELETE http://localhost:8080/api/tasks/0123456789abcdef
```

- 取消适用于排队或运行任务。
- 只有 `failed`、`canceled` 可重试。
- 只有 `succeeded` 可发布，并且服务端必须启用 AUR。
- 只有终态任务可删除；删除会同时清理该任务工作目录，返回 `204`。

## 文件下载

```bash
curl -b cookies.txt -OJ http://localhost:8080/api/tasks/0123456789abcdef/files/PKGBUILD
curl -b cookies.txt -OJ http://localhost:8080/api/tasks/0123456789abcdef/files/SRCINFO
curl -b cookies.txt -OJ http://localhost:8080/api/tasks/0123456789abcdef/files/build.log
curl -b cookies.txt -OJ http://localhost:8080/api/tasks/0123456789abcdef/files/agent.log
```

响应为 `text/plain; charset=utf-8`，文件名由 `Content-Disposition` 提供。

## Task 主要字段

| 字段 | 说明 |
| --- | --- |
| `id`、`source_url`、`package_name`、`version` | 任务和包信息 |
| `status` | 当前生命周期状态 |
| `attempts` | 已执行的容器构建次数 |
| `logs` | Agent 结构化日志数组 |
| `build_log` | 最近一次完整 makepkg 输出 |
| `pkgbuild`、`srcinfo` | 当前候选或验证通过的文件内容 |
| `error` | 当前错误，成功时通常为空 |
| `publication` | 仓库、分支、commit、包地址和发布时间 |
| `created_at`、`updated_at`、`finished_at` | 生命周期时间 |

全部字段和 schema 以 `openapi.yaml` 为准。

## Android 对接建议

1. 登录页收集服务器 URL 和密码；密码不写入 SharedPreferences、数据库或日志。
2. 使用系统 CookieManager 保存会话 Cookie，HttpURLConnection 自动附带。
3. 对 401 统一清理界面登录状态并返回登录页。
4. 后台轮询任务列表，选中任务可轮询详情或读取 SSE；不要在主线程进行网络请求。
5. Android 模拟器访问宿主机使用 `http://10.0.2.2:8080`；真机使用局域网地址或 HTTPS 域名。
6. 直接展示 PKGBUILD 和日志时使用等宽字体与纯文本，不要当作 HTML 渲染。

## 自建 AUR 对接

发布接口本身只接受任务 ID；仓库连接由服务端 `config.yaml` 管理：

- 标准 aurweb：使用 `ssh_host`、`ssh_port`、`ssh_user`、`private_key`。
- 非标准 SSH：使用 `repository_url: "ssh://git@example.com/{pkg}.git"`。
- HTTP(S) Git：使用 `repository_url: "https://git.example.com/aur/{pkg}.git"`、`git_username` 和环境变量 `AUR_GIT_TOKEN`。
- 测试/离线：使用本地路径或 `file://` 模板，例如 `/srv/aur/{pkg}.git`。

`{pkg}` 会替换为 URL 转义后的包名。token 通过临时 Askpass 环境变量传递，不进入 Git URL、进程参数或任务日志。
