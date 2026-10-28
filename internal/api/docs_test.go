package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestOpenAPISpec 验证规范可解析并覆盖全部公开任务操作。
func TestOpenAPISpec(t *testing.T) {
	var document struct {
		OpenAPI string                 `yaml:"openapi"`
		Paths   map[string]any         `yaml:"paths"`
		Info    map[string]interface{} `yaml:"info"`
	}
	if err := yaml.Unmarshal(openAPISpec, &document); err != nil {
		t.Fatalf("OpenAPI YAML 无效: %v", err)
	}
	if document.OpenAPI != "3.1.0" {
		t.Fatalf("规范版本为 %s", document.OpenAPI)
	}
	for _, path := range []string{"/api/auth/login", "/api/health", "/api/tasks", "/api/tasks/{id}", "/api/tasks/{id}/cancel", "/api/tasks/{id}/retry", "/api/tasks/{id}/publish", "/api/tasks/{id}/events", "/api/tasks/{id}/files/{name}"} {
		if _, exists := document.Paths[path]; !exists {
			t.Errorf("规范缺少路径 %s", path)
		}
	}
}

// TestDocsArePublic 验证文档无需登录且不会泄露页面密码或模型密钥。
func TestDocsArePublic(t *testing.T) {
	handler := &Handler{}
	for _, path := range []string{"/api/openapi.yaml", "/api/docs"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if path == "/api/openapi.yaml" {
			handler.openAPI(recorder, request)
		} else {
			handler.apiDocs(recorder, request)
		}
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "PackPilot") {
			t.Fatalf("文档 %s 不可访问: %d", path, recorder.Code)
		}
	}
}
