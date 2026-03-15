package onboarding

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// KeyCipher 提供 API Key 的加密/解密和指纹生成能力。
// 使用 AES-256-GCM 加密，HMAC-SHA256 生成不可逆指纹。
type KeyCipher struct {
	aead     cipher.AEAD
	hmacKey  []byte
	nonceLen int
}

// NewKeyCipher 从 hex 编码的 32 字节密钥创建 KeyCipher。
// 同一个密钥同时用于 AES-256-GCM 加密和 HMAC-SHA256 指纹（通过 KDF 派生不同子密钥）。
func NewKeyCipher(hexKey string) (*KeyCipher, error) {
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("解析加密密钥失败: %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("加密密钥长度必须为 32 字节（64 hex 字符），当前: %d 字节", len(keyBytes))
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	// 派生 HMAC 子密钥：HMAC-SHA256(masterKey, "fingerprint")
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte("fingerprint"))
	hmacKey := mac.Sum(nil)

	return &KeyCipher{
		aead:     aead,
		hmacKey:  hmacKey,
		nonceLen: aead.NonceSize(),
	}, nil
}

// Encrypt 加密明文 API Key，返回 hex 编码的 nonce+ciphertext。
func (kc *KeyCipher) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, kc.nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成 nonce 失败: %w", err)
	}

	ciphertext := kc.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt 解密 hex 编码的密文，返回明文 API Key。
func (kc *KeyCipher) Decrypt(hexCiphertext string) (string, error) {
	data, err := hex.DecodeString(hexCiphertext)
	if err != nil {
		return "", fmt.Errorf("解析密文失败: %w", err)
	}

	if len(data) < kc.nonceLen {
		return "", fmt.Errorf("密文长度不足")
	}

	nonce := data[:kc.nonceLen]
	ciphertext := data[kc.nonceLen:]

	plaintext, err := kc.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %w", err)
	}

	return string(plaintext), nil
}

// Fingerprint 生成 API Key 的不可逆指纹（HMAC-SHA256 hex）。
// 同一个 key 始终返回相同指纹，用于去重和匹配。
func (kc *KeyCipher) Fingerprint(apiKey string) string {
	mac := hmac.New(sha256.New, kc.hmacKey)
	mac.Write([]byte(apiKey))
	return hex.EncodeToString(mac.Sum(nil))
}

// Last4 返回 API Key 的最后 4 个字符（用于展示）。
func Last4(apiKey string) string {
	if len(apiKey) <= 4 {
		return apiKey
	}
	return apiKey[len(apiKey)-4:]
}
