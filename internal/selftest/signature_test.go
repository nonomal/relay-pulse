package selftest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSignatureValidatorGenerateMatchesManualHMAC(t *testing.T) {
	secret := "top-secret-key"
	validator := NewSignatureValidator(secret)

	timestamp := int64(1700000000)
	testType := "chat"
	apiURL := "https://api.example.com/v1/chat/completions"

	got := validator.GenerateSignature(timestamp, testType, apiURL)

	// 手动计算 HMAC-SHA256 验证
	message := fmt.Sprintf("%d:%s:%s", timestamp, testType, apiURL)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	want := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if got != want {
		t.Fatalf("GenerateSignature() = %q, want %q", got, want)
	}
}

func TestSignatureValidatorRoundTrip(t *testing.T) {
	validator := NewSignatureValidator("my-secret")

	timestamp := time.Now().Unix()
	testType := "chat"
	apiURL := "https://api.example.com/v1/chat/completions"

	signature := validator.GenerateSignature(timestamp, testType, apiURL)

	if err := validator.ValidateSignature(timestamp, testType, apiURL, signature); err != nil {
		t.Fatalf("ValidateSignature() round-trip failed: %v", err)
	}
}

func TestSignatureValidatorRejectsTampering(t *testing.T) {
	validator := NewSignatureValidator("my-secret")

	timestamp := time.Now().Unix()
	apiURL := "https://api.example.com/v1/chat/completions"
	validSig := validator.GenerateSignature(timestamp, "chat", apiURL)

	cases := []struct {
		name      string
		timestamp int64
		testType  string
		apiURL    string
		signature string
	}{
		{"tampered_signature", timestamp, "chat", apiURL, flipFirstChar(validSig)},
		{"wrong_test_type", timestamp, "vision", apiURL, validSig},
		{"wrong_api_url", timestamp, "chat", "https://evil.com/api", validSig},
		{"wrong_timestamp", timestamp + 1, "chat", apiURL, validSig},
		{"empty_signature", timestamp, "chat", apiURL, ""},
		{"wrong_secret", timestamp, "chat", apiURL,
			NewSignatureValidator("other-secret").GenerateSignature(timestamp, "chat", apiURL)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validator.ValidateSignature(tc.timestamp, tc.testType, tc.apiURL, tc.signature)
			if err == nil {
				t.Fatal("ValidateSignature() unexpectedly succeeded")
			}
			if !strings.Contains(err.Error(), "invalid signature") {
				t.Fatalf("error = %q, want 'invalid signature'", err.Error())
			}
		})
	}
}

func TestSignatureValidatorTimestampWindow(t *testing.T) {
	validator := NewSignatureValidator("my-secret")
	validator.SetTimestampWindow(3 * time.Second)

	apiURL := "https://api.example.com/v1/chat/completions"

	// 窗口内（过去 1s）
	pastOK := time.Now().Add(-1 * time.Second).Unix()
	sig := validator.GenerateSignature(pastOK, "chat", apiURL)
	if err := validator.ValidateSignature(pastOK, "chat", apiURL, sig); err != nil {
		t.Fatalf("past within window: unexpected error: %v", err)
	}

	// 窗口内（未来 1s）
	futureOK := time.Now().Add(1 * time.Second).Unix()
	sig = validator.GenerateSignature(futureOK, "chat", apiURL)
	if err := validator.ValidateSignature(futureOK, "chat", apiURL, sig); err != nil {
		t.Fatalf("future within window: unexpected error: %v", err)
	}

	// 窗口外（过去 10s）
	pastExpired := time.Now().Add(-10 * time.Second).Unix()
	sig = validator.GenerateSignature(pastExpired, "chat", apiURL)
	err := validator.ValidateSignature(pastExpired, "chat", apiURL, sig)
	if err == nil {
		t.Fatal("expired past timestamp: should have failed")
	}
	if !strings.Contains(err.Error(), "timestamp expired") {
		t.Fatalf("expired past error = %q, want 'timestamp expired'", err.Error())
	}

	// 窗口外（未来 10s）
	futureExpired := time.Now().Add(10 * time.Second).Unix()
	sig = validator.GenerateSignature(futureExpired, "chat", apiURL)
	err = validator.ValidateSignature(futureExpired, "chat", apiURL, sig)
	if err == nil {
		t.Fatal("expired future timestamp: should have failed")
	}
	if !strings.Contains(err.Error(), "timestamp expired") {
		t.Fatalf("expired future error = %q, want 'timestamp expired'", err.Error())
	}
}

func TestSignatureValidatorDefaultWindow(t *testing.T) {
	validator := NewSignatureValidator("my-secret")
	apiURL := "https://api.example.com/v1/chat/completions"

	// 默认窗口 5 分钟，4 分钟前的时间戳应通过
	ts := time.Now().Add(-4 * time.Minute).Unix()
	sig := validator.GenerateSignature(ts, "chat", apiURL)
	if err := validator.ValidateSignature(ts, "chat", apiURL, sig); err != nil {
		t.Fatalf("4min ago should be within default 5min window: %v", err)
	}

	// 6 分钟前的时间戳应失败
	tsExpired := time.Now().Add(-6 * time.Minute).Unix()
	sig = validator.GenerateSignature(tsExpired, "chat", apiURL)
	if err := validator.ValidateSignature(tsExpired, "chat", apiURL, sig); err == nil {
		t.Fatal("6min ago should be outside default 5min window")
	}
}

// flipFirstChar 翻转签名首字符以模拟篡改
func flipFirstChar(s string) string {
	if s == "" {
		return "A"
	}
	if s[0] == 'A' {
		return "B" + s[1:]
	}
	return "A" + s[1:]
}
