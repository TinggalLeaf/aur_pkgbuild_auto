package api

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var openAPISpec []byte

// openAPI 返回可供 Postman、Apifox 和代码生成器使用的 OpenAPI 3.1 文档。
func (h *Handler) openAPI(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	writer.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = writer.Write(openAPISpec)
}

// apiDocs 返回轻量中文文档首页，并链接机器可读规范和各接口分组。
func (h *Handler) apiDocs(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = writer.Write([]byte(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>PackPilot API 文档</title></head><body><main><h1>PackPilot API 1.1</h1><p>自主 Arch Linux 打包 Agent 的开放 HTTP API。</p><p><a href="/api/openapi.yaml">下载 OpenAPI 3.1 YAML</a></p><h2>认证</h2><p>POST /api/auth/login 获取 packpilot_session Cookie；除状态、登录和文档外均需认证。</p><h2>主要接口</h2><ul><li>GET /api/health：Docker 与并行 worker 状态</li><li>GET、POST /api/tasks：任务列表与创建</li><li>GET、DELETE /api/tasks/{id}：详情与删除</li><li>POST /api/tasks/{id}/cancel：取消</li><li>POST /api/tasks/{id}/retry：重试</li><li>POST /api/tasks/{id}/publish：发布到自建 AUR</li><li>GET /api/tasks/{id}/events：SSE 实时任务流</li><li>GET /api/tasks/{id}/files/{name}：PKGBUILD、SRCINFO、build.log、agent.log</li></ul><p>完整请求响应 schema、错误码和示例请查看 OpenAPI 文件。</p></main></body></html>`))
}
