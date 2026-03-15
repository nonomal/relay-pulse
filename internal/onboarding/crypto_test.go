package onboarding

import (
	"encoding/hex"
	"testing"
)

func testKey() string {
	// 32 bytes = 64 hex chars
	return "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}

func TestNewKeyCipher_ValidKey(t *testing.T) {
	kc, err := NewKeyCipher(testKey())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kc == nil {
		t.Fatal("expected non-nil KeyCipher")
	}
}

func TestNewKeyCipher_InvalidHex(t *testing.T) {
	_, err := NewKeyCipher("not-hex")
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

func TestNewKeyCipher_WrongLength(t *testing.T) {
	shortKey := hex.EncodeToString([]byte("tooshort"))
	_, err := NewKeyCipher(shortKey)
	if err == nil {
		t.Fatal("expected error for wrong length key")
	}
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	kc, err := NewKeyCipher(testKey())
	if err != nil {
		t.Fatalf("NewKeyCipher: %v", err)
	}

	plaintext := "sk-test-api-key-12345"
	encrypted, err := kc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if encrypted == plaintext {
		t.Error("encrypted should differ from plaintext")
	}

	decrypted, err := kc.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("got %q, want %q", decrypted, plaintext)
	}
}

func TestEncrypt_DifferentCiphertexts(t *testing.T) {
	kc, _ := NewKeyCipher(testKey())
	plaintext := "same-key"

	c1, _ := kc.Encrypt(plaintext)
	c2, _ := kc.Encrypt(plaintext)

	if c1 == c2 {
		t.Error("encrypting the same plaintext twice should produce different ciphertexts (random nonce)")
	}
}

func TestDecrypt_InvalidCiphertext(t *testing.T) {
	kc, _ := NewKeyCipher(testKey())

	_, err := kc.Decrypt("not-valid-hex")
	if err == nil {
		t.Error("expected error for invalid hex ciphertext")
	}

	_, err = kc.Decrypt(hex.EncodeToString([]byte("short")))
	if err == nil {
		t.Error("expected error for too-short ciphertext")
	}
}

func TestFingerprint_Deterministic(t *testing.T) {
	kc, _ := NewKeyCipher(testKey())

	f1 := kc.Fingerprint("my-api-key")
	f2 := kc.Fingerprint("my-api-key")

	if f1 != f2 {
		t.Error("fingerprint should be deterministic")
	}
}

func TestFingerprint_DifferentKeys(t *testing.T) {
	kc, _ := NewKeyCipher(testKey())

	f1 := kc.Fingerprint("key-a")
	f2 := kc.Fingerprint("key-b")

	if f1 == f2 {
		t.Error("different keys should produce different fingerprints")
	}
}

func TestLast4(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sk-1234567890", "7890"},
		{"abc", "abc"},
		{"", ""},
		{"1234", "1234"},
	}

	for _, tt := range tests {
		got := Last4(tt.input)
		if got != tt.want {
			t.Errorf("Last4(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
