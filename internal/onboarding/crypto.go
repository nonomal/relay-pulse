package onboarding

import "monitor/internal/apikey"

// KeyCipher 是 apikey.KeyCipher 的类型别名，保持向后兼容。
type KeyCipher = apikey.KeyCipher

// NewKeyCipher 从 hex 编码的 32 字节密钥创建 KeyCipher。
var NewKeyCipher = apikey.NewKeyCipher

// Last4 返回 API Key 的最后 4 个字符（用于展示）。
var Last4 = apikey.Last4
