# PackPilot — AUR PKGBUILD Agent

PackPilot 是一个 Go + Next.js 的自主 Arch Linux 打包工作台。输入 GitHub 仓库或软件官网后，Agent 会阅读上游页面与 releases、计算源码校验和、生成 PKGBUILD，在一次性 Arch Linux Docker 容器中运行 `makepkg`，根据失败日志迭代修复，并可推送到自建 aurweb。

## 功能

- 大模型工具循环：网页链接提取、GitHub releases、源码 SHA-256、候选包提交。
- 真实构建验证：`archlinux:base-devel`、非 root `makepkg`、依赖安装、`.SRCINFO` 生成。
- 构建安全边界：CPU、内存、PID、capabilities、执行时间和工作目录隔离。
- 任务持久化与恢复：JSON 快照、Agent 日志、构建日志、取消和 SSE 实时更新。
- 中文 Next.js 控制台：创建任务、状态筛选、日志、文件预览与下载、发布操作。
- 自建 aurweb：Git/SSH 克隆、提交和推送，服务端与任务级双重开关。

## 环境要求

- Go 1.27 或兼容版本
- Node.js 20.9+ 与 npm
- 可访问的 Docker 守护进程
- 支持 Anthropic Messages 工具调用协议的模型服务
- 如需发布：自建 aurweb、SSH 私钥和对应 Git 写权限

## 快速启动

1. 配置模型密钥、页面密码和独立会话密钥（不要写入仓库）。页面密码至少 12 位：

   ```bash
   export MINIMAX_API_KEY='你的密钥'
   export PACKPILOT_PASSWORD='请使用足够长的页面密码'
   export PACKPILOT_SESSION_SECRET='请使用随机生成的会话签名密钥'
   ```

2. 检查根目录 `config.yaml`。默认使用 `https://api.minimax.cn/anthropic` 和 `MiniMax-M2.5`，模型名可按账户实际支持情况调整。

3. 启动 Go API：

   ```bash
   go run ./cmd/server -config config.yaml
   ```

4. 另开终端启动 Next.js：

   ```bash
   cd public
   npm install
   npm run dev
   ```

5. 打开 <http://localhost:3000>，输入页面密码后粘贴上游 URL。Go API 默认位于 <http://localhost:8080>。

如 Go API 不在默认地址，在启动前端前设置：

```bash
export NEXT_PUBLIC_API_BASE_URL='http://localhost:8080'
```

## 配置

所有后端设置集中在 `config.yaml`，每项均有中文注释。`config.example.yaml` 是可复制模板。配置值支持 `${ENV_NAME}` 展开；模型密钥优先使用 `llm.api_key_env` 指向的环境变量。

主要分组：

| 分组 | 用途 |
| --- | --- |
| `server` | API 地址与允许的前端 Origin |
| `auth` | 页面密码、会话密钥、Cookie 安全和有效期 |
| `llm` | Anthropic 兼容地址、模型、token、轮次和超时 |
| `http` | 上游抓取超时、大小限制和 User-Agent |
| `worker` | 后台任务并发数 |
| `storage` | 任务与工作目录位置 |
| `docker` | Arch 镜像、CPU、内存、PID、网络和构建超时 |
| `aur` | aurweb 网页地址、SSH、Git 作者和发布开关 |

本地配置可保存为 `config.local.yaml`（已忽略），通过 `go run ./cmd/server -config config.local.yaml` 使用。

## 登录与公网安全

认证默认开启，未设置至少 12 位的 `PACKPILOT_PASSWORD` 时服务拒绝启动。所有任务、日志、健康状态和发布 API 都要求有效的 HMAC-SHA256 签名会话；登录失败按直接客户端地址限流。Cookie 使用 HttpOnly 和 SameSite=Lax，不写入 localStorage。

HTTPS 部署必须设置 `auth.cookie_secure: true`，并在反向代理启用 TLS、访问日志和额外速率限制。`auth.session_ttl` 默认为 24 小时。`auth.disabled: true` 仅用于明确受信的本地测试环境，不要在公网使用。

## 多任务与日志

`worker.concurrency` 控制同时运行的 Agent/容器数量，默认为 2。页面实时显示运行数、排队数和容量；每个任务有独立工作目录和取消函数，支持失败/取消后重试，以及终态任务和工作目录删除。

任务详情提供 SSE 实时日志、断线轮询回退、级别筛选、关键字搜索、自动滚动开关，并可下载 `agent.log`、完整 `build.log`、`PKGBUILD` 和 `.SRCINFO`。

## 国内网络与代理

代理同时覆盖 Agent 官网/GitHub 抓取、源码 SHA-256 下载、大模型请求，以及容器内 pacman 和 `makepkg` 的源码/依赖下载。支持 HTTP、HTTPS、SOCKS5，配置值可使用 `${HTTPS_PROXY}` 等环境变量。

以本机 Clash 的 `7890` 端口为例：

```yaml
http:
  # Go 进程访问代理；支持 socks5://127.0.0.1:7890。
  proxy_url: "http://127.0.0.1:7890"
  no_proxy: "localhost,127.0.0.1,::1"

docker:
  # 容器中的 127.0.0.1 是容器自己，因此使用宿主机映射名。
  proxy_url: "http://host.docker.internal:7890"
  no_proxy: "localhost,127.0.0.1,::1"
```

`docker.proxy_url` 留空时继承 `http.proxy_url`，`docker.no_proxy` 同理。程序自动为容器注入大小写 `HTTP_PROXY`、`HTTPS_PROXY`、`ALL_PROXY`、`NO_PROXY`，并添加 `host.docker.internal:host-gateway` 映射。代理软件需要允许来自 Docker 网段的连接；不要只监听不允许外部连接的环回地址。

NO_PROXY 使用逗号分隔，支持：

- `*`：全部直连，可用于容器显式绕过继承的代理。
- `example.com`、`.example.com`：匹配域名及其子域名。
- `127.0.0.1`、`::1`、`192.168.0.0/16`：匹配 IP 或 CIDR 网段。
- `example.com:8080`：只匹配指定端口。

未配置 `http.proxy_url` 时，Go 客户端继续尊重系统 `HTTP_PROXY`、`HTTPS_PROXY` 和 `NO_PROXY` 环境变量。带账号密码的代理 URL 应通过环境变量注入，避免提交到仓库。

## 自建 aurweb 发布

aurweb 的软件包仓库通常遵循 `ssh://aur@主机:端口/包名.git`。在 `config.yaml` 中设置：

```yaml
aur:
  enabled: true
  web_url: "https://aur.example.com"
  # 非标准部署可用模板覆盖，例如 "ssh://git@example.com/{pkg}.git"；通常留空。
  repository_url: ""
  ssh_user: "aur"
  ssh_host: "aur.example.com"
  ssh_port: 22
  private_key: "${AUR_SSH_PRIVATE_KEY}"
  known_hosts: "${AUR_KNOWN_HOSTS}"
  branch: "master"
  git_name: "Your Name"
  git_email: "you@example.com"
```

只有同时满足以下条件才会推送：

1. `aur.enabled: true`。
2. 任务已勾选自动发布，或用户在验证通过后点击“发布到 AUR”并再次确认。
3. `makepkg` 已成功且 `.SRCINFO` 已生成。

发布器仅写入 `PKGBUILD` 和 `.SRCINFO`。请先在测试 aurweb 实例验证权限与分支，不要直接对生产仓库开放不熟悉的密钥。

支持多种自建仓库连接：

- 标准 aurweb SSH：留空 `repository_url`，配置 SSH 主机和私钥。
- 自定义 SSH/本地仓库：使用包含 `{pkg}` 的完整模板，例如 `ssh://git@example.com/{pkg}.git` 或 `/srv/aur/{pkg}.git`。
- HTTP(S) Git：设置 `repository_url: "https://git.example.com/aur/{pkg}.git"`、`git_username` 和 `git_token_env: "AUR_GIT_TOKEN"`，token 通过环境变量提供。
- 已有仓库会先 fetch 并 rebase 目标分支；无文件变化时记录已有 commit，不产生空提交。

HTTP(S) token 使用权限 0700 的临时 Git Askpass 和环境变量传递，不进入 URL、命令参数或日志。

## API

完整对接文档见 `API.md`。运行服务后可直接访问：

- `GET /api/openapi.yaml`：OpenAPI 3.1 规范，可导入 Postman、Apifox 和代码生成器。
- `GET /api/docs`：轻量中文文档首页。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/auth/status` | 登录开关与当前会话状态 |
| POST | `/api/auth/login` | 页面密码登录并设置 Cookie |
| POST | `/api/auth/logout` | 退出并清理 Cookie |
| GET | `/api/health` | API、Docker 与 worker 并行状态（需登录） |
| GET | `/api/tasks` | 任务列表（需登录） |
| POST | `/api/tasks` | 创建任务 |
| GET | `/api/tasks/{id}` | 任务详情 |
| GET | `/api/tasks/{id}/events` | SSE 状态流 |
| POST | `/api/tasks/{id}/cancel` | 取消任务 |
| POST | `/api/tasks/{id}/retry` | 重试失败或取消任务 |
| DELETE | `/api/tasks/{id}` | 删除终态任务和工作目录 |
| POST | `/api/tasks/{id}/publish` | 发布已验证任务 |
| GET | `/api/tasks/{id}/files/PKGBUILD` | 下载 PKGBUILD |
| GET | `/api/tasks/{id}/files/SRCINFO` | 下载 .SRCINFO |
| GET | `/api/tasks/{id}/files/build.log` | 下载构建日志 |
| GET | `/api/tasks/{id}/files/agent.log` | 下载 Agent 执行日志 |

创建任务请求示例：

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H 'Content-Type: application/json' \
  -d '{"source_url":"https://github.com/owner/project","package_name":"","publish":false}'
```

## Android 客户端

`android/` 提供原生 Kotlin + Jetpack Compose 客户端，支持登录、并行状态、任务创建/取消/重试/删除/发布、日志与文件查看。

外观支持 MD3 / Miuix 切换、翡翠绿/海洋蓝/暮光紫/落日橙预设和跟随系统/浅色/深色模式。密码仅用于登录，不持久化。

```bash
cd android
./gradlew testDebugUnitTest lintDebug assembleDebug
```

APK 输出为 `android/app/build/outputs/apk/debug/app-debug.apk`。模拟器使用 `http://10.0.2.2:8080` 连接宿主机，详见 `android/README.md`。

## 目录结构

```text
cmd/server/             Go 服务入口
internal/agent/         大模型工具循环与构建修复
internal/api/           REST、SSE、CORS 和下载
internal/builder/       Arch Docker 构建器
internal/config/        YAML 加载、默认值和校验
internal/domain/        任务与发布模型
internal/llm/           Anthropic Messages 客户端
internal/publisher/     aurweb Git/SSH 发布器
internal/source/        网页、GitHub 和 SHA-256 工具
internal/store/         原子 JSON 持久化
internal/worker/        并发队列、恢复与取消
public/                 Next.js 中文控制台
```

## 检查命令

```bash
gofmt -w cmd internal
go test ./...
go test -race ./internal/...
go vet ./...
go build ./...
cd public
npm run lint
npm run build
```

## 安全说明

- 上游抓取仅允许 HTTP(S)，拒绝 URL 凭据、环回、私网和链路本地地址，并限制重定向与响应大小。
- PKGBUILD 本质上包含可执行脚本。容器隔离能降低风险，但不能替代对上游和生成内容的审计；不要在不信任的宿主机上开放高权限 Docker  socket。
- 构建容器默认需要网络下载源码和依赖。完全断网构建可设置 `docker.network: false`，但依赖必须已有其他供应方式。
- 页面密码和 API 认证默认开启，带登录限流、Origin 校验及安全响应头。公网部署仍应使用 HTTPS 反向代理，并增加访问审计和外部速率限制。
- 任务数据保存在 `data/`，包含上游地址、模型输出和构建日志，应按敏感运行数据保护。
