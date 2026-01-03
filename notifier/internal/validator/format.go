package validator

import "strings"

// FormatCandidates 格式化候选列表为用户友好的字符串
// kind: "service" 或 "channel"
func FormatCandidates(candidates []string, kind string) string {
	if len(candidates) == 0 {
		return ""
	}

	parts := make([]string, 0, len(candidates))
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		// 空 channel 显示为 "default"
		if c == "" && kind == "channel" {
			parts = append(parts, "default")
			continue
		}
		if c != "" {
			parts = append(parts, c)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "、")
}
