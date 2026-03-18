package selftest

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSafeHTTPClientDisablesProxy(t *testing.T) {
	client := newSafeHTTPClient(NewSSRFGuard())
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("Proxy should be nil to disable environment proxies")
	}
}

func TestSafeHTTPClientUsesColdStartSemantics(t *testing.T) {
	client := newSafeHTTPClient(NewSSRFGuard())
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T, want *http.Transport", client.Transport)
	}
	if !transport.DisableKeepAlives {
		t.Fatal("DisableKeepAlives = false, want true")
	}
	if transport.TLSNextProto == nil {
		t.Fatal("TLSNextProto = nil, want non-nil empty map to disable HTTP/2")
	}
	if len(transport.TLSNextProto) != 0 {
		t.Fatalf("TLSNextProto len = %d, want 0", len(transport.TLSNextProto))
	}
}

func TestSafeHTTPClientDisablesRedirects(t *testing.T) {
	client := newSafeHTTPClient(NewSSRFGuard())

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}
	if err := client.CheckRedirect(req, []*http.Request{req}); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("CheckRedirect = %v, want ErrUseLastResponse", err)
	}
}

func TestSafeHTTPClientDialContextRejectsLoopbackIP(t *testing.T) {
	// 在 127.0.0.1 上启动监听器，确保连接会被 SSRF 防护拦截
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen error: %v", err)
	}
	defer listener.Close()

	client := newSafeHTTPClient(NewSSRFGuard())
	transport := client.Transport.(*http.Transport)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, err := transport.DialContext(ctx, "tcp", listener.Addr().String())
	if err == nil {
		conn.Close()
		t.Fatal("DialContext should reject loopback IP")
	}
	if !strings.Contains(err.Error(), "blocked IP") {
		t.Fatalf("error = %q, want 'blocked IP'", err.Error())
	}
}

func TestSafeHTTPClientDialContextRejectsPrivateIP(t *testing.T) {
	client := newSafeHTTPClient(NewSSRFGuard())
	transport := client.Transport.(*http.Transport)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// 10.0.0.1:80 是私有 IP，连接请求应被拦截
	conn, err := transport.DialContext(ctx, "tcp", "10.0.0.1:80")
	if err == nil {
		conn.Close()
		t.Fatal("DialContext should reject private IP")
	}
	if !strings.Contains(err.Error(), "blocked IP") {
		t.Fatalf("error = %q, want 'blocked IP'", err.Error())
	}
}

func TestSafeHTTPClientDialContextRejectsLocalhostHostname(t *testing.T) {
	// localhost 解析后指向 127.0.0.1/::1，应被 SSRF 防护拦截
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen error: %v", err)
	}
	defer listener.Close()

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort error: %v", err)
	}

	client := newSafeHTTPClient(NewSSRFGuard())
	transport := client.Transport.(*http.Transport)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := transport.DialContext(ctx, "tcp", net.JoinHostPort("localhost", port))
	if err == nil {
		conn.Close()
		t.Fatal("DialContext should reject localhost hostname")
	}
	// 不同环境下错误信息可能不同
	errStr := err.Error()
	if !strings.Contains(errStr, "no public IPs available") &&
		!strings.Contains(errStr, "DNS lookup failed") &&
		!strings.Contains(errStr, "blocked") {
		t.Fatalf("error = %q, want localhost rejection", errStr)
	}
}

func TestSafeHTTPClientDialContextRejectsInvalidAddress(t *testing.T) {
	client := newSafeHTTPClient(NewSSRFGuard())
	transport := client.Transport.(*http.Transport)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, err := transport.DialContext(ctx, "tcp", "not-a-valid-address")
	if err == nil {
		conn.Close()
		t.Fatal("DialContext should reject invalid address")
	}
	if !strings.Contains(err.Error(), "invalid address") {
		t.Fatalf("error = %q, want 'invalid address'", err.Error())
	}
}
