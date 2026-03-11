package announcements

import (
	"fmt"
	"testing"
	"time"

	"monitor/internal/config"
)

func TestService_buildSnapshot(t *testing.T) {
	svc := &Service{
		cfg: config.AnnouncementsConfig{
			Owner:                "myorg",
			Repo:                 "myrepo",
			CategoryName:         "Announcements",
			WindowHours:          48,
			PollIntervalDuration: 5 * time.Minute,
			APIMaxAge:            120,
		},
	}

	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-48 * time.Hour)

	items := []Announcement{
		{
			ID: "D_1", Number: 42, Title: "重要公告",
			URL:       "https://github.com/myorg/myrepo/discussions/42",
			CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Author:    "admin",
		},
		{
			ID: "D_2", Number: 41, Title: "次要公告",
			URL:       "https://github.com/myorg/myrepo/discussions/41",
			CreatedAt: time.Date(2024, 1, 14, 8, 0, 0, 0, time.UTC),
			Author:    "mod",
		},
	}

	snap := svc.buildSnapshot(now, windowStart, items)

	// 基础字段
	if !snap.Enabled {
		t.Fatal("Enabled should be true")
	}
	if snap.APIMaxAge != 120 {
		t.Fatalf("APIMaxAge = %d, want 120", snap.APIMaxAge)
	}

	// Source
	if snap.Source.Provider != "github" {
		t.Fatalf("Source.Provider = %q", snap.Source.Provider)
	}
	if snap.Source.Repo != "myorg/myrepo" {
		t.Fatalf("Source.Repo = %q", snap.Source.Repo)
	}
	if snap.Source.Category != "Announcements" {
		t.Fatalf("Source.Category = %q", snap.Source.Category)
	}

	// Window
	if snap.Window.Hours != 48 {
		t.Fatalf("Window.Hours = %d, want 48", snap.Window.Hours)
	}
	if !snap.Window.StartAt.Equal(windowStart) {
		t.Fatalf("Window.StartAt = %v", snap.Window.StartAt)
	}
	if !snap.Window.EndAt.Equal(now) {
		t.Fatalf("Window.EndAt = %v", snap.Window.EndAt)
	}

	// Fetch
	if !snap.Fetch.FetchedAt.Equal(now) {
		t.Fatalf("Fetch.FetchedAt = %v", snap.Fetch.FetchedAt)
	}
	if snap.Fetch.Stale {
		t.Fatal("Fetch.Stale should be false")
	}
	if snap.Fetch.TTLSeconds != 300 {
		t.Fatalf("Fetch.TTLSeconds = %d, want 300", snap.Fetch.TTLSeconds)
	}

	// Items & Latest
	if len(snap.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(snap.Items))
	}
	if snap.Latest == nil {
		t.Fatal("Latest should not be nil")
	}
	if snap.Latest.Number != 42 {
		t.Fatalf("Latest.Number = %d, want 42", snap.Latest.Number)
	}

	// Version
	expectedVersion := fmt.Sprintf("%s#%d", items[0].CreatedAt.UTC().Format(time.RFC3339), 42)
	if snap.Version != expectedVersion {
		t.Fatalf("Version = %q, want %q", snap.Version, expectedVersion)
	}
}

func TestService_buildSnapshot_EmptyItems(t *testing.T) {
	svc := &Service{
		cfg: config.AnnouncementsConfig{
			Owner: "o", Repo: "r", CategoryName: "cat",
			PollIntervalDuration: time.Minute,
		},
	}

	now := time.Now()
	snap := svc.buildSnapshot(now, now.Add(-time.Hour), nil)

	if snap.Latest != nil {
		t.Fatalf("Latest should be nil for empty items, got %+v", snap.Latest)
	}
	if snap.Version != "" {
		t.Fatalf("Version should be empty for no items, got %q", snap.Version)
	}
	if len(snap.Items) != 0 {
		t.Fatalf("len(Items) = %d, want 0", len(snap.Items))
	}
}

func TestService_disabledSnapshot(t *testing.T) {
	fixedNow := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	svc := &Service{
		cfg: config.AnnouncementsConfig{
			Owner:          "o",
			Repo:           "r",
			CategoryName:   "cat",
			WindowHours:    24,
			WindowDuration: 24 * time.Hour,
		},
		nowFn: func() time.Time { return fixedNow },
	}

	snap := svc.disabledSnapshot()

	if snap.Enabled {
		t.Fatal("Enabled should be false")
	}
	if snap.APIMaxAge != 3600 {
		t.Fatalf("APIMaxAge = %d, want 3600", snap.APIMaxAge)
	}
	if len(snap.Items) != 0 {
		t.Fatalf("len(Items) = %d, want 0", len(snap.Items))
	}
	if snap.Source.Repo != "o/r" {
		t.Fatalf("Source.Repo = %q", snap.Source.Repo)
	}
	if snap.Window.Hours != 24 {
		t.Fatalf("Window.Hours = %d", snap.Window.Hours)
	}
}

func TestService_copySnapshotWithStale(t *testing.T) {
	svc := &Service{}

	original := &Snapshot{
		Enabled:   true,
		Version:   "v1",
		APIMaxAge: 60,
		Items: []Announcement{
			{ID: "D_1", Number: 1, Title: "test"},
		},
	}
	original.Fetch.Stale = false
	original.Fetch.FetchedAt = time.Now()
	original.Source.Provider = "github"
	latest := original.Items[0]
	original.Latest = &latest

	copy := svc.copySnapshotWithStale(original)

	// 应标记为 stale
	if !copy.Fetch.Stale {
		t.Fatal("copy should be stale")
	}

	// 原始不应受影响
	if original.Fetch.Stale {
		t.Fatal("original should not be modified")
	}

	// 其他字段应一致
	if copy.Enabled != original.Enabled {
		t.Fatal("Enabled mismatch")
	}
	if copy.Version != original.Version {
		t.Fatal("Version mismatch")
	}
	if len(copy.Items) != 1 {
		t.Fatalf("len(Items) = %d", len(copy.Items))
	}
}

func TestService_copySnapshotWithStale_Nil(t *testing.T) {
	svc := &Service{}
	if got := svc.copySnapshotWithStale(nil); got != nil {
		t.Fatalf("nil input should return nil, got %+v", got)
	}
}
