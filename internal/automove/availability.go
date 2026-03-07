package automove

import "monitor/internal/storage"

// CalculateAvailability 根据探测记录计算加权可用率百分比。
// 算法与 api/handler.go 中的 availabilityWeight() 保持一致：
//   - status=1（绿色）→ 权重 1.0
//   - status=2（黄色）→ 权重 degradedWeight
//   - status=0（红色）→ 权重 0.0
//
// 返回值：
//   - availability: 0-100 的百分比，无记录时返回 -1
//   - total: 参与计算的探测记录总数
func CalculateAvailability(records []*storage.ProbeRecord, degradedWeight float64) (availability float64, total int) {
	total = len(records)
	if total == 0 {
		return -1, 0
	}
	var weighted float64
	for _, r := range records {
		switch r.Status {
		case 1:
			weighted += 1.0
		case 2:
			weighted += degradedWeight
		}
	}
	availability = (weighted / float64(total)) * 100
	return availability, total
}
