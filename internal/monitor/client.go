package monitor

import (
	"net/http"
	"sync"
	"time"
)

// ClientPool HTTP客户端池（按provider管理，复用连接）
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

// GetClient 获取或创建客户端
func (p *ClientPool) GetClient(provider string) *http.Client {
	p.mu.RLock()
	client, exists := p.clients[provider]
	p.mu.RUnlock()

	if exists {
		return client
	}

	// 创建新客户端
	p.mu.Lock()
	defer p.mu.Unlock()

	// 双重检查
	if client, exists := p.clients[provider]; exists {
		return client
	}

	// 创建带连接池的HTTP客户端
	// 注意：不设置 Timeout，由 probe.go 使用 context.WithTimeout 控制每个请求的超时
	client = &http.Client{
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			DisableKeepAlives:   false,
		},
	}

	p.clients[provider] = client
	return client
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
