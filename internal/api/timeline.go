package api

import (
	"time"

	"monitor/internal/storage"
)

// bucketStats 用于聚合每个 bucket 内的探测数据
type bucketStats struct {
	total           int                  // 总探测次数
	weightedSuccess float64              // 累积成功权重（绿=1.0, 黄=degraded_weight, 红=0.0）
	latencySum      int64                // 延迟总和（仅统计可用状态）
	latencyCount    int                  // 有效延迟计数（仅 status > 0 的记录）
	allLatencySum   int64                // 所有记录延迟总和（用于全不可用时的参考）
	allLatencyCount int                  // 所有记录计数
	last            *storage.ProbeRecord // 最新一条记录
	statusCounts    storage.StatusCounts // 各状态计数
}

// buildTimeline 构建固定长度的时间轴，计算每个 bucket 的可用率和平均延迟
// endTime 为时间窗口的结束时间（对齐模式下为整点，动态模式下为当前时间）
// timeFilter 为每日时段过滤器，nil 表示全天（不过滤）
func (h *Handler) buildTimeline(records []*storage.ProbeRecord, endTime time.Time, period string, degradedWeight float64, timeFilter *TimeFilter) []storage.TimePoint {
	// 根据 period 确定 bucket 策略
	bucketCount, bucketWindow, format := h.determineBucketStrategy(period)

	// 90m 模式：不聚合，直接返回原始记录
	if bucketCount == 0 {
		return h.buildRawTimeline(records, endTime, format, degradedWeight, timeFilter)
	}

	// 使用传入的 endTime 作为基准时间（支持对齐模式）
	baseTime := endTime

	// 初始化 buckets 和统计数据
	buckets := make([]storage.TimePoint, bucketCount)
	stats := make([]bucketStats, bucketCount)

	for i := 0; i < bucketCount; i++ {
		bucketTime := baseTime.Add(-time.Duration(bucketCount-i) * bucketWindow)
		buckets[i] = storage.TimePoint{
			Time:         bucketTime.Format(format),
			Timestamp:    bucketTime.Unix(),
			Status:       -1, // 缺失标记
			Latency:      0,
			Availability: -1, // 缺失标记
		}
	}

	// 聚合每个 bucket 的探测结果
	for _, record := range records {
		t := time.Unix(record.Timestamp, 0)

		// 时段过滤：跳过不在指定时间段内的记录
		if timeFilter != nil && !timeFilter.Contains(t) {
			continue
		}

		timeDiff := baseTime.Sub(t)

		// 跳过超出时间窗口的记录：
		// - timeDiff < 0: 记录在 endTime 之后（对齐模式下当前小时的数据）
		// - timeDiff >= bucketCount * bucketWindow: 记录太旧
		if timeDiff < 0 {
			continue // 记录在窗口之后，跳过
		}

		// 计算该记录属于哪个 bucket（从后往前）
		bucketIndex := int(timeDiff / bucketWindow)
		if bucketIndex >= bucketCount {
			continue // 超出范围，忽略
		}

		// 从前往后的索引
		actualIndex := bucketCount - 1 - bucketIndex
		if actualIndex < 0 || actualIndex >= bucketCount {
			continue
		}

		// 聚合统计
		stat := &stats[actualIndex]
		stat.total++
		stat.weightedSuccess += availabilityWeight(record.Status, degradedWeight)
		// 统计所有记录的延迟（用于全不可用时的参考）
		if record.Latency > 0 {
			stat.allLatencySum += int64(record.Latency)
			stat.allLatencyCount++
		}
		// 只统计可用状态（status > 0）的延迟
		if record.Status > 0 {
			stat.latencySum += int64(record.Latency)
			stat.latencyCount++
		}
		incrementStatusCount(&stat.statusCounts, record.Status, record.SubStatus, record.HttpCode)

		// 保留最新记录
		if stat.last == nil || record.Timestamp > stat.last.Timestamp {
			stat.last = record
		}
	}

	// 根据聚合结果计算可用率和平均延迟
	for i := 0; i < bucketCount; i++ {
		stat := &stats[i]
		buckets[i].StatusCounts = stat.statusCounts
		if stat.total == 0 {
			continue
		}

		// 计算可用率（使用权重）
		buckets[i].Availability = (stat.weightedSuccess / float64(stat.total)) * 100

		// 计算平均延迟
		// 优先使用可用状态的延迟，若全部不可用则使用所有记录的延迟作为参考
		if stat.latencyCount > 0 {
			// 有可用记录：使用可用记录的平均延迟
			avgLatency := float64(stat.latencySum) / float64(stat.latencyCount)
			buckets[i].Latency = int(avgLatency + 0.5)
		} else if stat.allLatencyCount > 0 {
			// 全部不可用：使用所有记录的平均延迟作为参考（前端显示灰色）
			avgLatency := float64(stat.allLatencySum) / float64(stat.allLatencyCount)
			buckets[i].Latency = int(avgLatency + 0.5)
		}

		// 使用最新记录的状态
		if stat.last != nil {
			buckets[i].Status = stat.last.Status
			// 注意：Timestamp 保持为 bucket 起始时间，不覆盖
			// 这样前端可以准确显示时间段（如 03:00-04:00）
		}
	}

	return buckets
}

// buildTimelineFromAgg 使用 DB 返回的 bucket 聚合结果构建时间轴
//
// 约束：
// - 输出必须与 buildTimeline 完全一致（bucket 初始化、默认值、统计口径、取整规则）
// - rows 已在 DB 侧应用 timeFilter，本方法不再重复过滤
func (h *Handler) buildTimelineFromAgg(rows []storage.AggBucketRow, endTime time.Time, period string, degradedWeight float64) []storage.TimePoint {
	bucketCount, bucketWindow, format := h.determineBucketStrategy(period)

	// 90m 模式不走聚合（调用方应避免进入）
	if bucketCount == 0 {
		return make([]storage.TimePoint, 0)
	}

	baseTime := endTime

	// 初始化 buckets（与 buildTimeline 一致）
	buckets := make([]storage.TimePoint, bucketCount)
	for i := 0; i < bucketCount; i++ {
		bucketTime := baseTime.Add(-time.Duration(bucketCount-i) * bucketWindow)
		buckets[i] = storage.TimePoint{
			Time:         bucketTime.Format(format),
			Timestamp:    bucketTime.Unix(),
			Status:       -1,
			Latency:      0,
			Availability: -1,
		}
	}

	rowByIndex := make(map[int]storage.AggBucketRow, len(rows))
	for _, r := range rows {
		if r.BucketIndex < 0 || r.BucketIndex >= bucketCount {
			continue
		}
		rowByIndex[r.BucketIndex] = r
	}

	for i := 0; i < bucketCount; i++ {
		r, ok := rowByIndex[i]
		if !ok {
			continue
		}

		buckets[i].StatusCounts = r.StatusCounts
		if r.Total == 0 {
			continue
		}

		// 可用率：与 buildTimeline 的 availabilityWeight 累加等价（按计数计算）
		weightedSuccess := float64(r.StatusCounts.Available) + float64(r.StatusCounts.Degraded)*degradedWeight
		buckets[i].Availability = (weightedSuccess / float64(r.Total)) * 100

		// 平均延迟：取整规则与 buildTimeline 一致（+0.5 四舍五入）
		if r.LatencyCount > 0 {
			avgLatency := float64(r.LatencySum) / float64(r.LatencyCount)
			buckets[i].Latency = int(avgLatency + 0.5)
		} else if r.AllLatencyCount > 0 {
			avgLatency := float64(r.AllLatencySum) / float64(r.AllLatencyCount)
			buckets[i].Latency = int(avgLatency + 0.5)
		}

		// bucket 状态取"最后一条记录"的状态（Timestamp 仍保持 bucket 起始时间）
		buckets[i].Status = r.LastStatus
	}

	return buckets
}

// buildRawTimeline 将原始探测记录直接转换为时间轴（90m 模式专用，不聚合）
func (h *Handler) buildRawTimeline(records []*storage.ProbeRecord, endTime time.Time, format string, degradedWeight float64, timeFilter *TimeFilter) []storage.TimePoint {
	// 初始化为空切片，确保 JSON 序列化时返回 [] 而不是 null
	timeline := make([]storage.TimePoint, 0)

	for _, record := range records {
		t := time.Unix(record.Timestamp, 0)

		// 跳过 endTime 之后的记录（窗口之外）
		if t.After(endTime) {
			continue
		}

		// 时段过滤
		if timeFilter != nil && !timeFilter.Contains(t) {
			continue
		}

		// 构建状态计数（单条记录）
		var counts storage.StatusCounts
		incrementStatusCount(&counts, record.Status, record.SubStatus, record.HttpCode)

		// 延迟处理：始终返回原始延迟值，由前端决定颜色（可用=渐变，不可用=灰色）
		timeline = append(timeline, storage.TimePoint{
			Time:         t.UTC().Format(format),
			Timestamp:    record.Timestamp,
			Status:       record.Status,
			Latency:      record.Latency,
			Availability: statusToAvailability(record.Status, degradedWeight),
			StatusCounts: counts,
		})
	}

	return timeline
}

// determineBucketStrategy 根据 period 确定 bucket 数量、窗口大小和时间格式
// count=0 表示不聚合，返回原始记录
func (h *Handler) determineBucketStrategy(period string) (count int, window time.Duration, format string) {
	switch period {
	case "90m":
		return 0, 0, "15:04:05" // 不聚合，返回原始记录
	case "24h", "1d":
		return 24, time.Hour, "15:04"
	case "7d":
		return 7, 24 * time.Hour, "2006-01-02"
	case "30d":
		return 30, 24 * time.Hour, "2006-01-02"
	default:
		return 24, time.Hour, "15:04"
	}
}

// availabilityWeight 根据状态码返回可用率权重
func availabilityWeight(status int, degradedWeight float64) float64 {
	switch status {
	case 1: // 绿色（正常）
		return 1.0
	case 2: // 黄色（降级：如慢响应等）
		return degradedWeight
	default: // 红色（不可用）或灰色（未配置）
		return 0.0
	}
}

// statusToAvailability 将单条状态映射为可用率百分比（90m 模式专用）
func statusToAvailability(status int, degradedWeight float64) float64 {
	switch status {
	case 1: // 绿色
		return 100.0
	case 2: // 黄色
		return degradedWeight * 100
	case 0: // 红色
		return 0.0
	default: // 灰色（无数据）
		return -1
	}
}

// incrementStatusCount 统计每种状态及细分出现次数
func incrementStatusCount(counts *storage.StatusCounts, status int, subStatus storage.SubStatus, httpCode int) {
	switch status {
	case 1: // 绿色
		counts.Available++
	case 2: // 黄色
		counts.Degraded++
		// 黄色细分
		switch subStatus {
		case storage.SubStatusSlowLatency:
			counts.SlowLatency++
		case storage.SubStatusRateLimit:
			counts.RateLimit++
		}
	case 0: // 红色
		counts.Unavailable++
		// 红色细分
		switch subStatus {
		case storage.SubStatusRateLimit:
			// 限流现在视为红色不可用，但沿用 rate_limit 细分计数
			counts.RateLimit++
		case storage.SubStatusServerError:
			counts.ServerError++
		case storage.SubStatusClientError:
			counts.ClientError++
		case storage.SubStatusAuthError:
			counts.AuthError++
		case storage.SubStatusInvalidRequest:
			counts.InvalidRequest++
		case storage.SubStatusNetworkError:
			counts.NetworkError++
		case storage.SubStatusContentMismatch:
			counts.ContentMismatch++
		}
	default: // 灰色（3）或其他
		counts.Missing++
	}

	// 记录 HTTP 错误码细分（仅对红色状态且有有效 HTTP 响应码）
	if httpCode > 0 && status == 0 {
		// 仅统计与 HTTP 状态码相关的 SubStatus
		subKey := string(subStatus)
		switch subStatus {
		case storage.SubStatusServerError,
			storage.SubStatusClientError,
			storage.SubStatusAuthError,
			storage.SubStatusInvalidRequest,
			storage.SubStatusRateLimit:
			if counts.HttpCodeBreakdown == nil {
				counts.HttpCodeBreakdown = make(map[string]map[int]int)
			}
			if counts.HttpCodeBreakdown[subKey] == nil {
				counts.HttpCodeBreakdown[subKey] = make(map[int]int)
			}
			counts.HttpCodeBreakdown[subKey][httpCode]++
		}
	}
}
