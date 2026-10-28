// Package api 提供 Next.js 控制台使用的 REST 与 SSE 接口。
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aur_auto/internal/auth"
	"aur_auto/internal/builder"
	"aur_auto/internal/config"
	"aur_auto/internal/store"
	"aur_auto/internal/worker"
)

// Handler 组合任务存储、worker 和 Docker 健康检查。
type Handler struct {
	store   *store.Store
	worker  *worker.Manager
	builder *builder.Builder
	auth    *auth.Manager
	origin  string
	mux     *http.ServeMux
}

// createRequest 是创建任务时允许客户端提交的字段。
type createRequest struct {
	SourceURL   string `json:"source_url"`
	PackageName string `json:"package_name"`
	Publish     bool   `json:"publish"`
}

// New 注册全部 API 路由并返回带跨域控制的处理器。
func New(taskStore *store.Store, manager *worker.Manager, cfg *config.Config) http.Handler {
	handler := &Handler{
		store: taskStore, worker: manager, builder: builder.New(cfg.Docker),
		auth: auth.New(cfg.Auth), origin: cfg.Server.FrontendOrigin, mux: http.NewServeMux(),
	}
	handler.mux.HandleFunc("GET /api/auth/status", handler.authStatus)
	handler.mux.HandleFunc("POST /api/auth/login", handler.login)
	handler.mux.HandleFunc("POST /api/auth/logout", handler.logout)
	handler.mux.HandleFunc("GET /api/openapi.yaml", handler.openAPI)
	handler.mux.HandleFunc("GET /api/docs", handler.apiDocs)
	handler.mux.HandleFunc("GET /api/health", handler.health)
	handler.mux.HandleFunc("GET /api/tasks", handler.listTasks)
	handler.mux.HandleFunc("POST /api/tasks", handler.createTask)
	handler.mux.HandleFunc("GET /api/tasks/{id}", handler.getTask)
	handler.mux.HandleFunc("POST /api/tasks/{id}/cancel", handler.cancelTask)
	handler.mux.HandleFunc("POST /api/tasks/{id}/retry", handler.retryTask)
	handler.mux.HandleFunc("POST /api/tasks/{id}/publish", handler.publishTask)
	handler.mux.HandleFunc("DELETE /api/tasks/{id}", handler.deleteTask)
	handler.mux.HandleFunc("GET /api/tasks/{id}/events", handler.taskEvents)
	handler.mux.HandleFunc("GET /api/tasks/{id}/files/{name}", handler.downloadFile)
	return handler.security(handler.cors(handler.requireAuth(handler.mux)))
}

// health 返回 API、Docker 和后台 worker 池的实际状态。
func (h *Handler) health(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 16*time.Second)
	defer cancel()
	dockerStatus := "ok"
	if err := h.builder.Available(ctx); err != nil {
		dockerStatus = err.Error()
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"status": "ok", "docker": dockerStatus, "worker": h.worker.Stats(), "time": time.Now().UTC(),
	})
}

// listTasks 返回按时间倒序排列的任务列表。
func (h *Handler) listTasks(writer http.ResponseWriter, _ *http.Request) {
	tasks, err := h.store.List()
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"tasks": tasks})
}

// createTask 校验来源地址并创建可立即轮询的排队任务。
func (h *Handler) createTask(writer http.ResponseWriter, request *http.Request) {
	var input createRequest
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(writer, http.StatusBadRequest, fmt.Errorf("请求 JSON 无效: %w", err))
		return
	}
	parsed, err := url.Parse(strings.TrimSpace(input.SourceURL))
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		writeError(writer, http.StatusBadRequest, fmt.Errorf("请输入不带凭据的完整 HTTP(S) 地址"))
		return
	}
	input.PackageName = strings.TrimSpace(input.PackageName)
	if len(input.PackageName) > 128 {
		writeError(writer, http.StatusBadRequest, fmt.Errorf("包名过长"))
		return
	}
	task, err := h.store.Create(parsed.String(), input.PackageName, input.Publish)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	h.worker.Submit(task.ID)
	writeJSON(writer, http.StatusCreated, task)
}

// getTask 返回指定任务的完整状态快照。
func (h *Handler) getTask(writer http.ResponseWriter, request *http.Request) {
	task, err := h.store.Get(request.PathValue("id"))
	if err != nil {
		writeStoreError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, task)
}

// cancelTask 请求取消排队或执行中的任务。
func (h *Handler) cancelTask(writer http.ResponseWriter, request *http.Request) {
	if err := h.worker.Cancel(request.PathValue("id")); err != nil {
		writeStoreError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "canceled"})
}

// retryTask 将失败或取消的任务重新加入后台 worker 队列。
func (h *Handler) retryTask(writer http.ResponseWriter, request *http.Request) {
	if err := h.worker.Retry(request.PathValue("id")); err != nil {
		writeStoreError(writer, err)
		return
	}
	writeJSON(writer, http.StatusAccepted, map[string]string{"status": "queued"})
}

// deleteTask 删除已经结束的任务和其工作目录。
func (h *Handler) deleteTask(writer http.ResponseWriter, request *http.Request) {
	if err := h.worker.Delete(request.PathValue("id")); err != nil {
		writeStoreError(writer, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

// publishTask 请求将已验证任务发布到服务端配置的自建 aurweb。
func (h *Handler) publishTask(writer http.ResponseWriter, request *http.Request) {
	if err := h.worker.Publish(request.PathValue("id")); err != nil {
		writeStoreError(writer, err)
		return
	}
	writeJSON(writer, http.StatusAccepted, map[string]string{"status": "publishing"})
}

// taskEvents 通过 SSE 推送任务快照，并在不支持 SSE 时仍允许前端轮询详情接口。
func (h *Handler) taskEvents(writer http.ResponseWriter, request *http.Request) {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		writeError(writer, http.StatusInternalServerError, fmt.Errorf("服务器不支持 SSE"))
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()
	for {
		task, err := h.store.Get(request.PathValue("id"))
		if err != nil {
			return
		}
		data, err := json.Marshal(task)
		if err != nil {
			return
		}
		if _, err := fmt.Fprintf(writer, "event: task\ndata: %s\n\n", data); err != nil {
			return
		}
		flusher.Flush()
		if task.Terminal() {
			return
		}
		select {
		case <-request.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

// downloadFile 以附件形式返回已生成的 PKGBUILD、SRCINFO 或构建日志。
func (h *Handler) downloadFile(writer http.ResponseWriter, request *http.Request) {
	task, err := h.store.Get(request.PathValue("id"))
	if err != nil {
		writeStoreError(writer, err)
		return
	}
	var content, filename string
	switch request.PathValue("name") {
	case "PKGBUILD":
		content, filename = task.PKGBUILD, "PKGBUILD"
	case "SRCINFO":
		content, filename = task.SRCINFO, ".SRCINFO"
	case "build.log":
		content, filename = task.BuildLog, task.ID+"-build.log"
	case "agent.log":
		var logs strings.Builder
		for _, entry := range task.Logs {
			_, _ = fmt.Fprintf(&logs, "%s [%s] %s\n", entry.Time.Format(time.RFC3339), strings.ToUpper(entry.Level), entry.Message)
		}
		content, filename = logs.String(), task.ID+"-agent.log"
	default:
		writeError(writer, http.StatusNotFound, fmt.Errorf("未知文件"))
		return
	}
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	_, _ = writer.Write([]byte(content))
}

// cors 只允许配置的前端来源访问 API，并处理预检请求。
func (h *Handler) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if origin == h.origin {
			writer.Header().Set("Access-Control-Allow-Origin", h.origin)
			writer.Header().Set("Vary", "Origin")
			writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			writer.Header().Set("Access-Control-Allow-Credentials", "true")
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		}
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

// writeStoreError 将任务不存在和其他存储错误映射为合适的 HTTP 状态。
func writeStoreError(writer http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(writer, http.StatusNotFound, err)
		return
	}
	writeError(writer, http.StatusBadRequest, err)
}

// writeError 以统一 JSON 结构返回不含敏感配置的错误。
func writeError(writer http.ResponseWriter, status int, err error) {
	writeJSON(writer, status, map[string]string{"error": err.Error()})
}

// writeJSON 设置 UTF-8 JSON 响应头并编码响应体。
func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
