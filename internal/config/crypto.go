package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	// ConfigEncryptionKeyEnv API Key 加解密密钥环境变量名
	// 支持两种格式：
	//   1. 单密钥：Base64 编码的 32 字节密钥（自动使用版本 1）
	//   2. 多版本：version:base64,version:base64,... 用于密钥轮换
	ConfigEncryptionKeyEnv = "CONFIG_ENCRYPTION_KEY"

	// aes256KeySize AES-256 密钥长度（32 字节）
	aes256KeySize = 32

	// encVersionAES256GCM 当前加密算法版本
	encVersionAES256GCM = 1
)

// encryptionKeySet 加密密钥集合（支持多版本用于轮换）
type encryptionKeySet struct {
	DefaultVersion int            // 加密时使用的默认版本（最高版本号）
	Keys           map[int][]byte // version -> 密钥
}

var (
	keySetMu     sync.RWMutex
	cachedKeySet *encryptionKeySet
	cachedKeyEnv string // 用于检测环境变量是否变化
)

// EncryptionResult API Key 加密结果
type EncryptionResult struct {
	Ciphertext []byte
	Nonce      []byte
	KeyVersion int
	EncVersion int
}

// EncryptAPIKey 使用 AES-256-GCM 加密 API Key
//
// 参数：
//   - apiKey: 待加密的 API Key 明文
//   - monitorID: 监测项 ID，作为 AAD (Additional Authenticated Data)
//   - keyVersion: 指定密钥版本，<=0 时使用默认版本（最高版本）
//
// 返回：
//   - 加密结果（密文、nonce、使用的密钥版本、加密算法版本）
//   - 错误信息
//
// AAD 绑定说明：
// monitorID 作为 AAD 参与 GCM 认证，确保密文只能用于对应的监测项。
// 如果尝试用错误的 monitorID 解密，会因 tag 校验失败而报错。
func EncryptAPIKey(apiKey string, monitorID int64, keyVersion int) (*EncryptionResult, error) {
	if apiKey == "" {
		return nil, errors.New("apiKey 不能为空")
	}

	key, usedVersion, err := resolveEncryptionKey(keyVersion)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("初始化 AES 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("初始化 GCM 失败: %w", err)
	}

	// 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成随机 nonce 失败: %w", err)
	}

	// 使用 monitorID 作为 AAD
	aad := monitorIDToAAD(monitorID)

	// 加密（GCM 会自动附加认证 tag）
	ciphertext := gcm.Seal(nil, nonce, []byte(apiKey), aad)

	return &EncryptionResult{
		Ciphertext: ciphertext,
		Nonce:      nonce,
		KeyVersion: usedVersion,
		EncVersion: encVersionAES256GCM,
	}, nil
}

// DecryptAPIKey 使用 AES-256-GCM 解密 API Key
//
// 参数：
//   - ciphertext: 密文
//   - nonce: 加密时使用的 nonce
//   - monitorID: 监测项 ID，必须与加密时一致
//   - keyVersion: 加密时使用的密钥版本
//
// 返回：
//   - 解密后的 API Key 明文
//   - 错误信息
func DecryptAPIKey(ciphertext, nonce []byte, monitorID int64, keyVersion int) (string, error) {
	if len(ciphertext) == 0 {
		return "", errors.New("密文为空")
	}
	if len(nonce) == 0 {
		return "", errors.New("nonce 为空")
	}
	if keyVersion <= 0 {
		return "", errors.New("keyVersion 必须 > 0")
	}

	// 解密时必须使用加密时的密钥版本
	key, _, err := resolveEncryptionKey(keyVersion)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("初始化 AES 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("初始化 GCM 失败: %w", err)
	}

	if len(nonce) != gcm.NonceSize() {
		return "", fmt.Errorf("nonce 长度无效，期望 %d 字节，实际 %d 字节", gcm.NonceSize(), len(nonce))
	}

	// 使用 monitorID 作为 AAD
	aad := monitorIDToAAD(monitorID)

	// 解密并验证认证 tag
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return "", fmt.Errorf("解密失败（可能是密钥不匹配或 monitorID 错误）: %w", err)
	}

	return string(plaintext), nil
}

// IsEncryptionKeyConfigured 检查是否配置了加密密钥
func IsEncryptionKeyConfigured() bool {
	env := strings.TrimSpace(os.Getenv(ConfigEncryptionKeyEnv))
	return env != ""
}

// ValidateEncryptionKey 验证加密密钥配置是否有效
// 返回 nil 表示有效，否则返回具体错误
func ValidateEncryptionKey() error {
	if !IsEncryptionKeyConfigured() {
		return fmt.Errorf("未配置环境变量 %s", ConfigEncryptionKeyEnv)
	}
	_, err := loadEncryptionKeySet()
	return err
}

// GetEncryptionKeyVersions 获取所有可用的密钥版本
func GetEncryptionKeyVersions() ([]int, error) {
	keySet, err := loadEncryptionKeySet()
	if err != nil {
		return nil, err
	}
	versions := make([]int, 0, len(keySet.Keys))
	for v := range keySet.Keys {
		versions = append(versions, v)
	}
	return versions, nil
}

// GetDefaultKeyVersion 获取默认密钥版本
func GetDefaultKeyVersion() (int, error) {
	keySet, err := loadEncryptionKeySet()
	if err != nil {
		return 0, err
	}
	return keySet.DefaultVersion, nil
}

// MaskAPIKey 对 API Key 进行脱敏处理
// 格式：前 3 位 + "..." + 后 4 位（如 "sk-...abcd"）
// 若长度不足则返回 "***"
func MaskAPIKey(apiKey string) string {
	if len(apiKey) < 10 {
		return "***"
	}
	prefix := apiKey[:3]
	suffix := apiKey[len(apiKey)-4:]
	return prefix + "..." + suffix
}

// resolveEncryptionKey 解析指定版本的加密密钥
// keyVersion <= 0 时返回默认版本（最高版本）
func resolveEncryptionKey(keyVersion int) ([]byte, int, error) {
	keySet, err := loadEncryptionKeySet()
	if err != nil {
		return nil, 0, err
	}

	if keyVersion <= 0 {
		keyVersion = keySet.DefaultVersion
	}

	key, ok := keySet.Keys[keyVersion]
	if !ok {
		return nil, 0, fmt.Errorf("未找到 key_version=%d 对应的密钥", keyVersion)
	}

	return key, keyVersion, nil
}

// loadEncryptionKeySet 加载并缓存加密密钥集合
// 使用环境变量值作为缓存 key，环境变量变化时自动重新解析
func loadEncryptionKeySet() (*encryptionKeySet, error) {
	env := strings.TrimSpace(os.Getenv(ConfigEncryptionKeyEnv))
	if env == "" {
		return nil, fmt.Errorf("未配置环境变量 %s", ConfigEncryptionKeyEnv)
	}

	// 快速路径：检查缓存
	keySetMu.RLock()
	if cachedKeySet != nil && env == cachedKeyEnv {
		ks := cachedKeySet
		keySetMu.RUnlock()
		return ks, nil
	}
	keySetMu.RUnlock()

	// 解析密钥配置
	ks, err := parseEncryptionKeySet(env)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	keySetMu.Lock()
	cachedKeySet = ks
	cachedKeyEnv = env
	keySetMu.Unlock()

	return ks, nil
}

// parseEncryptionKeySet 解析加密密钥配置字符串
// 支持格式：
//  1. 单密钥：Base64 编码的 32 字节密钥
//  2. 多版本：v1:base64,v2:base64 或 1:base64,2:base64
func parseEncryptionKeySet(raw string) (*encryptionKeySet, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("未配置环境变量 %s", ConfigEncryptionKeyEnv)
	}

	// 检测格式：单密钥 vs 多版本
	// 多版本格式必须包含 , 或者以 "数字:" 或 "v数字:" 开头
	isMultiVersion := strings.Contains(raw, ",") || looksLikeVersionedKey(raw)

	if !isMultiVersion {
		// 单密钥格式：直接是 Base64 编码的密钥
		key, err := decodeBase64Key(raw)
		if err != nil {
			return nil, fmt.Errorf("解析 %s 失败: %w", ConfigEncryptionKeyEnv, err)
		}
		return &encryptionKeySet{
			DefaultVersion: 1,
			Keys:           map[int][]byte{1: key},
		}, nil
	}

	// 多版本格式：v1:base64,v2:base64
	entries := strings.Split(raw, ",")
	keys := make(map[int][]byte)
	maxVersion := 0

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		version, keyRaw, err := splitKeyEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("解析 %s 失败: %w", ConfigEncryptionKeyEnv, err)
		}

		key, err := decodeBase64Key(keyRaw)
		if err != nil {
			return nil, fmt.Errorf("解析 %s (版本 %d) 失败: %w", ConfigEncryptionKeyEnv, version, err)
		}

		keys[version] = key
		if version > maxVersion {
			maxVersion = version
		}
	}

	if len(keys) == 0 {
		return nil, errors.New("未解析到有效的密钥配置")
	}

	return &encryptionKeySet{
		DefaultVersion: maxVersion,
		Keys:           keys,
	}, nil
}

// looksLikeVersionedKey 检查字符串是否看起来像版本化密钥格式
// 版本化格式以 "数字:" 或 "v数字:" 开头
func looksLikeVersionedKey(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	// 检查是否以 v 开头
	if strings.HasPrefix(strings.ToLower(s), "v") {
		s = s[1:]
	}

	// 找到第一个非数字字符
	for i, c := range s {
		if c == ':' || c == '=' {
			// 如果前面有数字，则是版本化格式
			return i > 0
		}
		if c < '0' || c > '9' {
			// 非数字且非分隔符，不是版本化格式
			return false
		}
	}

	// 全是数字，不是版本化格式
	return false
}

// splitKeyEntry 解析单个密钥条目
// 支持格式：v1:base64 或 1:base64 或 v1=base64 或 1=base64
func splitKeyEntry(entry string) (int, string, error) {
	var versionPart, keyPart string

	switch {
	case strings.Contains(entry, ":"):
		versionPart, keyPart, _ = strings.Cut(entry, ":")
	case strings.Contains(entry, "="):
		versionPart, keyPart, _ = strings.Cut(entry, "=")
	default:
		return 0, "", errors.New("密钥格式无效，期望 version:base64 或 version=base64")
	}

	// 解析版本号，支持 v1 或 1 格式
	versionPart = strings.TrimSpace(versionPart)
	versionPart = strings.TrimPrefix(strings.ToLower(versionPart), "v")
	if versionPart == "" {
		return 0, "", errors.New("缺少密钥版本号")
	}

	version, err := strconv.Atoi(versionPart)
	if err != nil || version <= 0 {
		return 0, "", fmt.Errorf("密钥版本号无效: %s（必须为正整数）", versionPart)
	}

	keyPart = strings.TrimSpace(keyPart)
	if keyPart == "" {
		return 0, "", errors.New("缺少密钥内容")
	}

	return version, keyPart, nil
}

// decodeBase64Key 解码 Base64 格式的密钥
// 支持标准 Base64 和 Raw Base64（无 padding）
func decodeBase64Key(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("密钥为空")
	}

	// 尝试标准 Base64
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		// 尝试 Raw Base64（无 padding）
		key, err = base64.RawStdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("Base64 解码失败: %w", err)
		}
	}

	if len(key) != aes256KeySize {
		return nil, fmt.Errorf("密钥长度必须为 %d 字节（Base64 编码后约 %d 字符），当前 %d 字节",
			aes256KeySize, aes256KeySize*4/3+4, len(key))
	}

	return key, nil
}

// monitorIDToAAD 将 monitorID 转换为 AAD
func monitorIDToAAD(monitorID int64) []byte {
	return []byte(strconv.FormatInt(monitorID, 10))
}

// ClearEncryptionKeyCache 清除加密密钥缓存
// 仅供测试使用
func ClearEncryptionKeyCache() {
	keySetMu.Lock()
	cachedKeySet = nil
	cachedKeyEnv = ""
	keySetMu.Unlock()
}
