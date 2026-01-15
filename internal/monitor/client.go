package monitor

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// ClientPool HTTP客户端池（按 provider+proxy 组合管理，复用连接）
type ClientPool struct {
	mu      sync.RWMutex
	clients map[string]*http.Client
}

// NewClientPool 创建客户端池
func NewClientPool() *ClientPool {
	return &ClientPool{
		clients: make(map[string]*http.Client),
	}
}

// clientKey 生成客户端缓存键
// 相同 provider 和 proxy 组合复用同一个客户端
func clientKey(provider, proxyURL string) string {
	if proxyURL == "" {
		return provider
	}
	return fmt.Sprintf("%s|%s", provider, proxyURL)
}

// GetClient 获取或创建客户端
// proxyURL 为空时使用系统环境变量代理
func (p *ClientPool) GetClient(provider, proxyURL string) (*http.Client, error) {
	key := clientKey(provider, proxyURL)

	p.mu.RLock()
	client, exists := p.clients[key]
	p.mu.RUnlock()

	if exists {
		return client, nil
	}

	// 创建新客户端
	p.mu.Lock()
	defer p.mu.Unlock()

	// 双重检查
	if client, exists := p.clients[key]; exists {
		return client, nil
	}

	// 创建 Transport
	transport, err := createTransport(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("创建 Transport 失败: %w", err)
	}

	// 创建带连接池的HTTP客户端
	// 注意：不设置 Timeout，由 probe.go 使用 context.WithTimeout 控制每个请求的超时
	client = &http.Client{
		Transport: transport,
	}

	p.clients[key] = client
	return client, nil
}

// createTransport 创建 HTTP Transport，支持代理配置
// proxyURL 为空时使用系统环境变量代理
func createTransport(proxyURL string) (http.RoundTripper, error) {
	baseTransport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}

	// 无自定义代理时，使用系统环境变量
	if proxyURL == "" {
		baseTransport.Proxy = http.ProxyFromEnvironment
		return baseTransport, nil
	}

	// 解析代理 URL
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("解析代理 URL 失败: %w", err)
	}

	// 防御性处理：scheme 小写化（配置层已做规范化，这里是兜底）
	scheme := strings.ToLower(parsed.Scheme)

	switch scheme {
	case "http", "https":
		// HTTP/HTTPS 代理：直接设置 Proxy 函数
		baseTransport.Proxy = http.ProxyURL(parsed)
		return baseTransport, nil

	case "socks5", "socks":
		// SOCKS5 代理：使用 golang.org/x/net/proxy
		return createSOCKS5Transport(parsed, baseTransport)

	default:
		return nil, fmt.Errorf("不支持的代理协议: %s（支持 http, https, socks5, socks）", parsed.Scheme)
	}
}

// createSOCKS5Transport 创建 SOCKS5 代理的 Transport
func createSOCKS5Transport(proxyURL *url.URL, baseTransport *http.Transport) (*http.Transport, error) {
	// 提取认证信息
	var auth *proxy.Auth
	if proxyURL.User != nil {
		auth = &proxy.Auth{
			User: proxyURL.User.Username(),
		}
		if password, ok := proxyURL.User.Password(); ok {
			auth.Password = password
		}
	}

	// 创建 SOCKS5 dialer
	dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, auth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("创建 SOCKS5 dialer 失败: %w", err)
	}

	// 设置 DialContext（需要类型断言，因为 proxy.SOCKS5 返回的是 Dialer 接口）
	if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
		baseTransport.DialContext = contextDialer.DialContext
	} else {
		// 回退到非 context 版本
		baseTransport.Dial = dialer.Dial //nolint:staticcheck // 兼容老版本
	}

	return baseTransport, nil
}

// Close 关闭所有客户端
func (p *ClientPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, client := range p.clients {
		client.CloseIdleConnections()
	}

	p.clients = make(map[string]*http.Client)
}
