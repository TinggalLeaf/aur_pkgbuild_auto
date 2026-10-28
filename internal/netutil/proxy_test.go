package netutil

import (
	"net/http"
	"net/url"
	"testing"
)

// TestTransportProxyAndBypass 验证显式代理、域名排除和环境回退行为。
func TestTransportProxyAndBypass(t *testing.T) {
	transport, err := Transport("http://127.0.0.1:7890", "example.com,192.168.0.0/16")
	if err != nil {
		t.Fatal(err)
	}
	proxied := httptestRequest("https://github.com/owner/repo")
	proxy, err := transport.Proxy(proxied)
	if err != nil || proxy.String() != "http://127.0.0.1:7890" {
		t.Fatalf("未使用显式代理: %v %v", proxy, err)
	}
	for _, raw := range []string{"https://example.com/file", "https://cdn.example.com/file", "http://192.168.1.8/file"} {
		proxy, err := transport.Proxy(httptestRequest(raw))
		if err != nil || proxy != nil {
			t.Fatalf("NO_PROXY 未绕过 %s: %v", raw, proxy)
		}
	}
}

// TestBypassRules 覆盖通配符、CIDR 和指定端口规则。
func TestBypassRules(t *testing.T) {
	for _, test := range []struct{ target, rule string }{
		{"https://anything.example/path", "*"},
		{"http://10.2.3.4/path", "10.0.0.0/8"},
		{"http://internal.example:8080/path", "internal.example:8080"},
	} {
		parsed, _ := url.Parse(test.target)
		if !bypassProxy(parsed, test.rule) {
			t.Fatalf("规则 %s 未匹配 %s", test.rule, test.target)
		}
	}
}

// TestInvalidProxy 验证不支持的代理协议会在启动前被拒绝。
func TestInvalidProxy(t *testing.T) {
	if _, err := Transport("ftp://127.0.0.1:21", ""); err == nil {
		t.Fatal("无效代理协议未被拒绝")
	}
}

// httptestRequest 构造只用于代理决策的 HTTP 请求。
func httptestRequest(raw string) *http.Request {
	request, _ := http.NewRequest(http.MethodGet, raw, nil)
	return request
}
