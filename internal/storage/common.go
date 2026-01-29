// Package storage 提供数据存储相关的公共工具函数
package storage

// reverseRecords 反转记录数组（DESC 取数后翻转为时间升序）
func reverseRecords(records []*ProbeRecord) {
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}
}

// normalizeLimitOffset 规范化分页参数（从 MonitorConfigFilter 提取）
// limit <= 0 表示不限制，offset < 0 会被重置为 0
func normalizeLimitOffset(filter *MonitorConfigFilter) (int, int) {
	if filter == nil {
		return -1, 0
	}
	limit := filter.Limit
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = -1 // 不限制
	}
	return limit, offset
}
