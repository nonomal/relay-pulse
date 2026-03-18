package monitor

import (
	"net/http"
	"testing"
)

func TestCreateTransportDisablesConnectionReuse(t *testing.T) {
	t.Parallel()

	rt, err := createTransport("")
	if err != nil {
		t.Fatalf("createTransport returned error: %v", err)
	}

	transport, ok := rt.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", rt)
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

func TestCreateTransportWithHTTPProxyKeepsColdStartSemantics(t *testing.T) {
	t.Parallel()

	rt, err := createTransport("http://proxy.example.com:8080")
	if err != nil {
		t.Fatalf("createTransport returned error: %v", err)
	}

	transport, ok := rt.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", rt)
	}
	if transport.Proxy == nil {
		t.Fatal("Proxy = nil, want proxy function")
	}
	if !transport.DisableKeepAlives {
		t.Fatal("DisableKeepAlives = false, want true")
	}
	if transport.TLSNextProto == nil || len(transport.TLSNextProto) != 0 {
		t.Fatal("TLSNextProto should be a non-nil empty map")
	}
}
