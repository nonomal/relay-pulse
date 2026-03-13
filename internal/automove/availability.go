package automove

import (
	"time"

	"monitor/internal/storage"
)

const (
	availabilityBucketCount  = 7
	availabilityBucketWindow = 24 * time.Hour

	bucketWindowSeconds = int64(availabilityBucketWindow / time.Second)
)

// CalculateAvailability 根据探测记录计算 7 天加权可用率百分比。
// 算法与 WebUI 的 bucket 聚合逻辑保持一致：
//  1. 以 endTime 为右端点，向前切分 7 个 24h bucket
//  2. 每个 bucket 内按状态权重计算可用率
//  3. 仅对非空 bucket 求等权平均（空 bucket 跳过）
//
// endTime 应使用 alignToNextUTCDay() 对齐到下一天 00:00 UTC，
// 与 api/query.go 中 7d period 的 day 对齐保持一致。
//
// 返回值：
//   - availability: 0-100 的百分比，无有效记录时返回 -1
//   - total: 实际落入 bucket 并参与计算的探测记录总数
func CalculateAvailability(records []*storage.ProbeRecord, endTime time.Time, degradedWeight float64) (availability float64, total int) {
	if len(records) == 0 {
		return -1, 0
	}

	endUnix := endTime.UTC().Unix()

	var buckets [availabilityBucketCount]struct {
		total    int
		weighted float64
	}

	for _, r := range records {
		if r == nil {
			continue
		}
		age := endUnix - r.Timestamp
		if age < 0 {
			continue // 未来记录，跳过
		}
		idx := int(age / bucketWindowSeconds)
		if idx >= availabilityBucketCount {
			continue // 窗口外记录，跳过
		}
		buckets[idx].total++
		buckets[idx].weighted += statusWeight(r.Status, degradedWeight)
		total++
	}

	if total == 0 {
		return -1, 0
	}

	var sum float64
	var nonEmpty int
	for _, b := range buckets {
		if b.total == 0 {
			continue
		}
		sum += (b.weighted / float64(b.total)) * 100
		nonEmpty++
	}

	return sum / float64(nonEmpty), total
}

// statusWeight 返回探测状态对应的可用率权重。
func statusWeight(status int, degradedWeight float64) float64 {
	switch status {
	case 1:
		return 1.0
	case 2:
		return degradedWeight
	default:
		return 0.0
	}
}
