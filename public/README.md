# PackPilot 前端

这是 PackPilot 的 Next.js 16 中文控制台，与项目根目录的 Go API 配合使用。

## 启动

先在项目根目录启动 Go 服务，然后运行：

```bash
npm install
npm run dev
```

打开 <http://localhost:3000>。前端默认请求 `http://localhost:8080`，可通过 `NEXT_PUBLIC_API_BASE_URL` 修改。

## 页面能力

- 页面密码登录、HttpOnly 签名会话、401 自动返回登录页和退出登录。
- 提交 GitHub 仓库或软件官网，可选指定包名和自动发布。
- 查看并行 worker 容量、任务统计、状态筛选和包名/仓库/ID 搜索。
- 实时日志支持级别筛选、关键字搜索、自动滚动开关和 Agent 日志下载。
- 预览并下载 PKGBUILD、`.SRCINFO` 和完整构建日志。
- 取消、重试和删除任务；对验证通过的软件包确认后发布到自建 aurweb。
- 优先使用携带 Cookie 的 SSE 接收任务更新，连接失败时自动回退到轮询。

## 检查

```bash
npm run lint
npm run build
```

完整配置、后端启动和部署说明见项目根目录 `README.md`。
