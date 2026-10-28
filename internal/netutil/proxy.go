// Package netutil 提供 Agent 各网络客户端共用的代理传输配置。
package netutil

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
)

// Transport 创建支持显式代理和 NO_PROXY 的 HTTP 传输层；未配置代理时继承系统环境。
func Transport(proxyURL, noProxy string) (*http.Transport, error) {
	var parsedProxy *url.URL
	if strings.TrimSpace(proxyURL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(proxyURL))
		if err != nil || parsed.Host == "" {
			return nil, fmt.Errorf("代理 URL 无效")
		}
		switch parsed.Scheme {
		case "http", "https", "socks5":
		default:
			return nil, fmt.Errorf("代理仅支持 http、https 和 socks5 协议")
		}
		parsedProxy = parsed
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = func(request *http.Request) (*url.URL, error) {
		if bypassProxy(request.URL, noProxy) {
			return nil, nil
		}
		if parsedProxy != nil {
			return parsedProxy, nil
		}
		return http.ProxyFromEnvironment(request)
	}
	return transport, nil
}

// bypassProxy 判断目标是否命中通配符、域名、IP 或 CIDR 格式的 NO_PROXY 规则。
func bypassProxy(target *url.URL, noProxy string) bool {
	for _, item := range strings.Split(noProxy, ",") {
		rule := strings.TrimSpace(item)
		if rule == "" {
			continue
		}
		if rule == "*" {
			return true
		}
		if matchIPRule(target.Hostname(), rule) || matchDomainRule(target, rule) {
			return true
		}
	}
	return false
}

// matchIPRule 匹配字面 IP 和 CIDR 排除规则。
func matchIPRule(host, rule string) bool {
	address, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return false
	}
	if prefix, err := netip.ParsePrefix(rule); err == nil {
		return prefix.Contains(address)
	}
	ruleAddress, err := netip.ParseAddr(strings.Trim(rule, "[]"))
	return err == nil && address == ruleAddress
}

// matchDomainRule 匹配域名、点前缀后缀以及可选端口规则。
func matchDomainRule(target *url.URL, rule string) bool {
	host := strings.ToLower(strings.TrimSuffix(target.Hostname(), "."))
	rule = strings.ToLower(strings.TrimSpace(rule))
	if strings.Contains(rule, ":") && !strings.HasPrefix(rule, "[") {
		ruleHost, rulePort, err := net.SplitHostPort(rule)
		if err != nil || target.Port() != rulePort {
			return false
		}
		rule = ruleHost
	}
	rule = strings.TrimPrefix(strings.TrimSuffix(rule, "."), ".")
	return rule != "" && (host == rule || strings.HasSuffix(host, "."+rule))
}
