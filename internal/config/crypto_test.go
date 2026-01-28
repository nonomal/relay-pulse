package config

import (
	"encoding/base64"
	"os"
	"testing"
)

// generateTestKey 生成测试用的 32 字节密钥
func generateTestKey() string {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestEncryptDecryptAPIKey(t *testing.T) {
	// 设置测试密钥
	testKey := generateTestKey()
	os.Setenv(ConfigEncryptionKeyEnv, testKey)
	defer os.Unsetenv(ConfigEncryptionKeyEnv)
	ClearEncryptionKeyCache()

	tests := []struct {
		name      string
		apiKey    string
		monitorID int64
	}{
		{"简单密钥", "sk-test-12345", 1},
		{"长密钥", "sk-very-long-api-key-with-many-characters-1234567890", 100},
		{"特殊字符", "sk-test!@#$%^&*()_+-=[]{}|;':\",./<>?", 999},
		{"空格和换行", "sk-test with spaces\nand\nnewlines", 42},
		{"Unicode", "sk-测试密钥-🔑", 888},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 加密
			result, err := EncryptAPIKey(tt.apiKey, tt.monitorID, 0)
			if err != nil {
				t.Fatalf("加密失败: %v", err)
			}

			if len(result.Ciphertext) == 0 {
				t.Error("密文为空")
			}
			if len(result.Nonce) == 0 {
				t.Error("nonce 为空")
			}
			if result.KeyVersion != 1 {
				t.Errorf("密钥版本错误，期望 1，实际 %d", result.KeyVersion)
			}
			if result.EncVersion != encVersionAES256GCM {
				t.Errorf("加密算法版本错误，期望 %d，实际 %d", encVersionAES256GCM, result.EncVersion)
			}

			// 解密
			decrypted, err := DecryptAPIKey(result.Ciphertext, result.Nonce, tt.monitorID, result.KeyVersion)
			if err != nil {
				t.Fatalf("解密失败: %v", err)
			}

			if decrypted != tt.apiKey {
				t.Errorf("解密结果不匹配\n期望: %q\n实际: %q", tt.apiKey, decrypted)
			}
		})
	}
}

func TestDecryptWithWrongMonitorID(t *testing.T) {
	testKey := generateTestKey()
	os.Setenv(ConfigEncryptionKeyEnv, testKey)
	defer os.Unsetenv(ConfigEncryptionKeyEnv)
	ClearEncryptionKeyCache()

	apiKey := "sk-test-12345"
	monitorID := int64(100)

	// 加密
	result, err := EncryptAPIKey(apiKey, monitorID, 0)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	// 使用错误的 monitorID 解密，应该失败
	wrongMonitorID := int64(999)
	_, err = DecryptAPIKey(result.Ciphertext, result.Nonce, wrongMonitorID, result.KeyVersion)
	if err == nil {
		t.Error("使用错误的 monitorID 解密应该失败")
	}
}

func TestMultiVersionKeys(t *testing.T) {
	// 生成两个不同的密钥
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i)
		key2[i] = byte(i + 100)
	}

	// 设置多版本密钥
	multiKeyConfig := "v1:" + base64.StdEncoding.EncodeToString(key1) +
		",v2:" + base64.StdEncoding.EncodeToString(key2)
	os.Setenv(ConfigEncryptionKeyEnv, multiKeyConfig)
	defer os.Unsetenv(ConfigEncryptionKeyEnv)
	ClearEncryptionKeyCache()

	// 验证默认使用最高版本
	defaultVersion, err := GetDefaultKeyVersion()
	if err != nil {
		t.Fatalf("获取默认版本失败: %v", err)
	}
	if defaultVersion != 2 {
		t.Errorf("默认版本应为 2，实际为 %d", defaultVersion)
	}

	// 获取所有版本
	versions, err := GetEncryptionKeyVersions()
	if err != nil {
		t.Fatalf("获取版本列表失败: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("应有 2 个版本，实际 %d 个", len(versions))
	}

	apiKey := "sk-test-12345"
	monitorID := int64(100)

	// 使用 v1 加密
	resultV1, err := EncryptAPIKey(apiKey, monitorID, 1)
	if err != nil {
		t.Fatalf("v1 加密失败: %v", err)
	}
	if resultV1.KeyVersion != 1 {
		t.Errorf("期望使用 v1，实际 v%d", resultV1.KeyVersion)
	}

	// 使用 v2 加密
	resultV2, err := EncryptAPIKey(apiKey, monitorID, 2)
	if err != nil {
		t.Fatalf("v2 加密失败: %v", err)
	}
	if resultV2.KeyVersion != 2 {
		t.Errorf("期望使用 v2，实际 v%d", resultV2.KeyVersion)
	}

	// 默认版本应使用 v2
	resultDefault, err := EncryptAPIKey(apiKey, monitorID, 0)
	if err != nil {
		t.Fatalf("默认版本加密失败: %v", err)
	}
	if resultDefault.KeyVersion != 2 {
		t.Errorf("默认版本应为 v2，实际 v%d", resultDefault.KeyVersion)
	}

	// 使用对应版本解密
	decrypted1, err := DecryptAPIKey(resultV1.Ciphertext, resultV1.Nonce, monitorID, 1)
	if err != nil {
		t.Fatalf("v1 解密失败: %v", err)
	}
	if decrypted1 != apiKey {
		t.Error("v1 解密结果不匹配")
	}

	decrypted2, err := DecryptAPIKey(resultV2.Ciphertext, resultV2.Nonce, monitorID, 2)
	if err != nil {
		t.Fatalf("v2 解密失败: %v", err)
	}
	if decrypted2 != apiKey {
		t.Error("v2 解密结果不匹配")
	}

	// 使用错误版本解密应失败
	_, err = DecryptAPIKey(resultV1.Ciphertext, resultV1.Nonce, monitorID, 2)
	if err == nil {
		t.Error("使用错误版本解密应失败")
	}
}

func TestKeyVersionFormats(t *testing.T) {
	key := generateTestKey()

	tests := []struct {
		name    string
		config  string
		wantErr bool
	}{
		{"单密钥", key, false},
		{"v前缀", "v1:" + key, false},
		{"无v前缀", "1:" + key, false},
		{"等号分隔", "1=" + key, false},
		{"多版本逗号分隔", "v1:" + key + ",v2:" + key, false},
		{"空格", "  v1:" + key + "  ", false},
		{"无效版本号", "v0:" + key, true},
		{"负版本号", "v-1:" + key, true},
		{"非数字版本", "va:" + key, true},
		{"空密钥", "", true},
		{"只有版本号", "v1:", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(ConfigEncryptionKeyEnv, tt.config)
			defer os.Unsetenv(ConfigEncryptionKeyEnv)
			ClearEncryptionKeyCache()

			err := ValidateEncryptionKey()
			if tt.wantErr && err == nil {
				t.Error("期望返回错误，但返回了 nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("不期望错误，但返回了: %v", err)
			}
		})
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sk-1234567890abcdef", "sk-...cdef"},
		{"short", "***"},
		{"", "***"},
		{"12345678", "***"},
		{"1234567890", "123...7890"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := MaskAPIKey(tt.input)
			if result != tt.expected {
				t.Errorf("MaskAPIKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsEncryptionKeyConfigured(t *testing.T) {
	// 清理环境
	os.Unsetenv(ConfigEncryptionKeyEnv)
	ClearEncryptionKeyCache()

	if IsEncryptionKeyConfigured() {
		t.Error("未配置密钥时应返回 false")
	}

	os.Setenv(ConfigEncryptionKeyEnv, generateTestKey())
	defer os.Unsetenv(ConfigEncryptionKeyEnv)

	if !IsEncryptionKeyConfigured() {
		t.Error("已配置密钥时应返回 true")
	}
}

func TestEncryptEmptyAPIKey(t *testing.T) {
	testKey := generateTestKey()
	os.Setenv(ConfigEncryptionKeyEnv, testKey)
	defer os.Unsetenv(ConfigEncryptionKeyEnv)
	ClearEncryptionKeyCache()

	_, err := EncryptAPIKey("", 1, 0)
	if err == nil {
		t.Error("加密空 API Key 应返回错误")
	}
}

func TestDecryptInvalidInput(t *testing.T) {
	testKey := generateTestKey()
	os.Setenv(ConfigEncryptionKeyEnv, testKey)
	defer os.Unsetenv(ConfigEncryptionKeyEnv)
	ClearEncryptionKeyCache()

	tests := []struct {
		name       string
		ciphertext []byte
		nonce      []byte
		monitorID  int64
		keyVersion int
	}{
		{"空密文", nil, make([]byte, 12), 1, 1},
		{"空nonce", []byte("test"), nil, 1, 1},
		{"无效版本", []byte("test"), make([]byte, 12), 1, 0},
		{"不存在的版本", []byte("test"), make([]byte, 12), 1, 999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecryptAPIKey(tt.ciphertext, tt.nonce, tt.monitorID, tt.keyVersion)
			if err == nil {
				t.Error("期望返回错误")
			}
		})
	}
}

func TestKeyCache(t *testing.T) {
	key1 := generateTestKey()
	os.Setenv(ConfigEncryptionKeyEnv, key1)
	defer os.Unsetenv(ConfigEncryptionKeyEnv)
	ClearEncryptionKeyCache()

	// 首次加载
	err := ValidateEncryptionKey()
	if err != nil {
		t.Fatalf("首次验证失败: %v", err)
	}

	// 再次验证应使用缓存
	err = ValidateEncryptionKey()
	if err != nil {
		t.Fatalf("缓存验证失败: %v", err)
	}

	// 修改环境变量后应重新加载
	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = byte(i + 50)
	}
	os.Setenv(ConfigEncryptionKeyEnv, base64.StdEncoding.EncodeToString(key2))

	err = ValidateEncryptionKey()
	if err != nil {
		t.Fatalf("新密钥验证失败: %v", err)
	}
}

func TestRandomnessOfNonce(t *testing.T) {
	testKey := generateTestKey()
	os.Setenv(ConfigEncryptionKeyEnv, testKey)
	defer os.Unsetenv(ConfigEncryptionKeyEnv)
	ClearEncryptionKeyCache()

	apiKey := "sk-test-12345"
	monitorID := int64(1)

	// 多次加密同一内容，nonce 应不同
	nonces := make(map[string]bool)
	for i := 0; i < 100; i++ {
		result, err := EncryptAPIKey(apiKey, monitorID, 0)
		if err != nil {
			t.Fatalf("加密失败: %v", err)
		}
		nonceStr := string(result.Nonce)
		if nonces[nonceStr] {
			t.Error("检测到重复的 nonce，随机性不足")
		}
		nonces[nonceStr] = true
	}
}
