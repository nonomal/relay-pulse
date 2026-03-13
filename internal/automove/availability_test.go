package automove

import (
	"testing"
	"time"

	"monitor/internal/storage"
)

// refTime 返回一个固定的 endTime，用于测试。
// 使用 2026-03-14 00:00 UTC（已对齐到 day 边界）。
func refTime() time.Time {
	return time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
}

// tsAt 返回 refTime 之前 hoursAgo 小时处的 Unix 时间戳。
func tsAt(hoursAgo int) int64 {
	return refTime().Add(-time.Duration(hoursAgo) * time.Hour).Unix()
}

func TestCalculateAvailability_Empty(t *testing.T) {
	avail, total := CalculateAvailability(nil, refTime(), 0.7)
	if total != 0 {
		t.Errorf("expected total=0, got %d", total)
	}
	if avail != -1 {
		t.Errorf("expected availability=-1, got %f", avail)
	}
}

func TestCalculateAvailability_AllGreen(t *testing.T) {
	records := make([]*storage.ProbeRecord, 10)
	for i := range records {
		records[i] = &storage.ProbeRecord{Status: 1, Timestamp: tsAt(i + 1)}
	}
	avail, total := CalculateAvailability(records, refTime(), 0.7)
	if total != 10 {
		t.Errorf("expected total=10, got %d", total)
	}
	if avail != 100.0 {
		t.Errorf("expected availability=100.0, got %f", avail)
	}
}

func TestCalculateAvailability_AllRed(t *testing.T) {
	records := make([]*storage.ProbeRecord, 5)
	for i := range records {
		records[i] = &storage.ProbeRecord{Status: 0, Timestamp: tsAt(i + 1)}
	}
	avail, total := CalculateAvailability(records, refTime(), 0.7)
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if avail != 0.0 {
		t.Errorf("expected availability=0.0, got %f", avail)
	}
}

func TestCalculateAvailability_Mixed(t *testing.T) {
	// 7 green + 3 yellow, 全部在同一 bucket 内（day 0）
	// bucket 可用率 = (7*1.0 + 3*0.7) / 10 * 100 = 91.0
	// 只有 1 个非空 bucket → 总可用率 = 91.0
	records := make([]*storage.ProbeRecord, 10)
	for i := 0; i < 7; i++ {
		records[i] = &storage.ProbeRecord{Status: 1, Timestamp: tsAt(i + 1)}
	}
	for i := 7; i < 10; i++ {
		records[i] = &storage.ProbeRecord{Status: 2, Timestamp: tsAt(i + 1)}
	}
	avail, total := CalculateAvailability(records, refTime(), 0.7)
	if total != 10 {
		t.Errorf("expected total=10, got %d", total)
	}
	expected := 91.0
	if avail < expected-0.01 || avail > expected+0.01 {
		t.Errorf("expected availability≈%.1f, got %f", expected, avail)
	}
}

func TestCalculateAvailability_MixedWithRed(t *testing.T) {
	// 5 green + 3 yellow + 2 red, 全部在同一 bucket 内
	// bucket 可用率 = (5*1.0 + 3*0.5 + 2*0) / 10 * 100 = 65.0
	records := []*storage.ProbeRecord{
		{Status: 1, Timestamp: tsAt(1)}, {Status: 1, Timestamp: tsAt(2)},
		{Status: 1, Timestamp: tsAt(3)}, {Status: 1, Timestamp: tsAt(4)},
		{Status: 1, Timestamp: tsAt(5)},
		{Status: 2, Timestamp: tsAt(6)}, {Status: 2, Timestamp: tsAt(7)},
		{Status: 2, Timestamp: tsAt(8)},
		{Status: 0, Timestamp: tsAt(9)}, {Status: 0, Timestamp: tsAt(10)},
	}
	avail, _ := CalculateAvailability(records, refTime(), 0.5)
	expected := 65.0
	if avail < expected-0.01 || avail > expected+0.01 {
		t.Errorf("expected availability≈%.1f, got %f", expected, avail)
	}
}

func TestCalculateAvailability_AveragesNonEmptyBuckets(t *testing.T) {
	// 核心场景：探测密度不均导致旧算法和新算法结果不同
	// Day 0 (最近一天): 1 条绿色 → bucket 可用率 = 100%
	// Day 1: 9 条红色 → bucket 可用率 = 0%
	// Day 2-6: 无数据 → 跳过
	// 新算法: (100 + 0) / 2 = 50%
	// 旧算法: (1*1.0 + 9*0.0) / 10 * 100 = 10%
	end := refTime()
	records := make([]*storage.ProbeRecord, 10)
	// 1 green in day 0 (0-24h ago)
	records[0] = &storage.ProbeRecord{Status: 1, Timestamp: end.Add(-1 * time.Hour).Unix()}
	// 9 red in day 1 (24-48h ago)
	for i := 1; i < 10; i++ {
		records[i] = &storage.ProbeRecord{
			Status:    0,
			Timestamp: end.Add(-time.Duration(24+i) * time.Hour).Unix(),
		}
	}

	avail, total := CalculateAvailability(records, end, 0.7)
	if total != 10 {
		t.Errorf("expected total=10, got %d", total)
	}
	expected := 50.0
	if avail < expected-0.01 || avail > expected+0.01 {
		t.Errorf("expected availability≈%.1f (bucket average), got %f", expected, avail)
	}
}

func TestCalculateAvailability_OutsideWindowIgnored(t *testing.T) {
	end := refTime()
	records := []*storage.ProbeRecord{
		// 8 天前 → 超出 7 bucket 窗口，应被跳过
		{Status: 0, Timestamp: end.Add(-8 * 24 * time.Hour).Unix()},
		// 1 小时前 → 在窗口内
		{Status: 1, Timestamp: end.Add(-1 * time.Hour).Unix()},
	}

	avail, total := CalculateAvailability(records, end, 0.7)
	if total != 1 {
		t.Errorf("expected total=1 (outside-window record skipped), got %d", total)
	}
	if avail != 100.0 {
		t.Errorf("expected availability=100.0, got %f", avail)
	}
}

func TestCalculateAvailability_FutureRecordIgnored(t *testing.T) {
	end := refTime()
	records := []*storage.ProbeRecord{
		{Status: 0, Timestamp: end.Add(1 * time.Hour).Unix()}, // future
		{Status: 1, Timestamp: end.Add(-1 * time.Hour).Unix()},
	}

	avail, total := CalculateAvailability(records, end, 0.7)
	if total != 1 {
		t.Errorf("expected total=1 (future record skipped), got %d", total)
	}
	if avail != 100.0 {
		t.Errorf("expected availability=100.0, got %f", avail)
	}
}

func TestCalculateAvailability_EndTimeBoundary(t *testing.T) {
	// 记录时间戳恰好等于 endTime → age=0, bucket index=0 → 应计入
	end := refTime()
	records := []*storage.ProbeRecord{
		{Status: 1, Timestamp: end.Unix()},
	}

	avail, total := CalculateAvailability(records, end, 0.7)
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if avail != 100.0 {
		t.Errorf("expected availability=100.0, got %f", avail)
	}
}

func TestCalculateAvailability_Exactly24hAgo_Bucket1(t *testing.T) {
	// age = 24h 整 → bucketIndex = 24*3600 / (24*3600) = 1 → bucket 1
	end := refTime()
	records := []*storage.ProbeRecord{
		{Status: 1, Timestamp: end.Add(-24 * time.Hour).Unix()},
	}
	avail, total := CalculateAvailability(records, end, 0.7)
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if avail != 100.0 {
		t.Errorf("expected availability=100.0, got %f", avail)
	}
}

func TestCalculateAvailability_Exactly7dAgo_Excluded(t *testing.T) {
	// age = 7*24h 整 → bucketIndex = 7 → >= availabilityBucketCount → 被排除
	end := refTime()
	records := []*storage.ProbeRecord{
		{Status: 1, Timestamp: end.Add(-7 * 24 * time.Hour).Unix()},
	}
	avail, total := CalculateAvailability(records, end, 0.7)
	if total != 0 {
		t.Errorf("expected total=0 (7d boundary excluded), got %d", total)
	}
	if avail != -1 {
		t.Errorf("expected availability=-1, got %f", avail)
	}
}

func TestCalculateAvailability_JustBefore7d_Bucket6(t *testing.T) {
	// age = 7*24h - 1s → bucketIndex = 6 → 最后一个有效 bucket
	end := refTime()
	records := []*storage.ProbeRecord{
		{Status: 0, Timestamp: end.Add(-7*24*time.Hour + time.Second).Unix()},
	}
	avail, total := CalculateAvailability(records, end, 0.7)
	if total != 1 {
		t.Errorf("expected total=1 (just inside 7d window), got %d", total)
	}
	if avail != 0.0 {
		t.Errorf("expected availability=0.0, got %f", avail)
	}
}

func TestCalculateAvailability_MultipleBuckets(t *testing.T) {
	// 7 个 bucket 各有 1 条记录：bucket 0-2 绿色，bucket 3-6 红色
	// 可用率 = (100*3 + 0*4) / 7 ≈ 42.86%
	end := refTime()
	records := make([]*storage.ProbeRecord, 7)
	for i := 0; i < 7; i++ {
		status := 0
		if i < 3 {
			status = 1
		}
		records[i] = &storage.ProbeRecord{
			Status:    status,
			Timestamp: end.Add(-time.Duration(i*24+1) * time.Hour).Unix(),
		}
	}

	avail, total := CalculateAvailability(records, end, 0.7)
	if total != 7 {
		t.Errorf("expected total=7, got %d", total)
	}
	expected := 300.0 / 7.0
	if avail < expected-0.01 || avail > expected+0.01 {
		t.Errorf("expected availability≈%.2f, got %f", expected, avail)
	}
}
