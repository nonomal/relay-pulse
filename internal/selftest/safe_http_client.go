package selftest

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// newSafeHTTPClient 创建自助测试专用的安全 HTTP 客户端：
// - 禁用重定向（避免 3xx 跳转绕过 SSRF 校验）
// - 自定义 DialContext：在实际连接时校验目标 IP（防 DNS rebinding）
// - 禁用环境代理（避免通过代理访问内网资源）
func newSafeHTTPClient(guard *SSRFGuard) *http.Client {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy: nil, // 禁用代理，避免绕过 SSRF 防护
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address: %w", err)
			}

			// 先做一次解析校验（防止解析直接指向内网）
			if ip := net.ParseIP(host); ip != nil {
				// 直接是 IP 地址
				if !ip.IsGlobalUnicast() || guard.isPrivateIP(ip) {
					return nil, fmt.Errorf("blocked IP: %s", ip.String())
				}
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err != nil {
					return nil, err
				}
				// 连接后再次校验 remote IP（兜底抵御 DNS rebinding/解析差异）
				if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
					if tcpAddr.IP == nil || !tcpAddr.IP.IsGlobalUnicast() || guard.isPrivateIP(tcpAddr.IP) {
						_ = conn.Close()
						return nil, fmt.Errorf("blocked remote IP: %s", tcpAddr.IP.String())
					}
				}
				return conn, nil
			}

			// 域名需要先解析
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, fmt.Errorf("DNS lookup failed for %s: %w", host, err)
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("DNS lookup returned no IP addresses for %s", host)
			}

			// 尝试连接每个解析出的 IP（仅尝试公网 IP）
			var lastErr error
			for _, ip := range ips {
				if ip == nil || !ip.IsGlobalUnicast() || guard.isPrivateIP(ip) {
					continue
				}
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err != nil {
					lastErr = err
					continue
				}
				// 连接成功后再次校验 remote IP
				if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
					if tcpAddr.IP == nil || !tcpAddr.IP.IsGlobalUnicast() || guard.isPrivateIP(tcpAddr.IP) {
						_ = conn.Close()
						lastErr = fmt.Errorf("blocked remote IP: %s", tcpAddr.IP.String())
						continue
					}
				}
				return conn, nil
			}

			if lastErr != nil {
				return nil, lastErr
			}
			return nil, fmt.Errorf("no public IPs available for host: %s", host)
		},
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       30 * time.Second,
		DisableKeepAlives:     false,
		ForceAttemptHTTP2:     true,
	}

	return &http.Client{
		Transport: transport,
		// 禁用自动重定向（避免跳转到内网绕过 SSRF 校验）
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
