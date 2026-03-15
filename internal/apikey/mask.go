package apikey

// Last4 返回 API Key 的最后 4 个字符（用于展示）。
func Last4(apiKey string) string {
	if len(apiKey) <= 4 {
		return apiKey
	}
	return apiKey[len(apiKey)-4:]
}
