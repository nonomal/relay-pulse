package automove

import (
	"testing"

	"monitor/internal/storage"
)

func TestCalculateAvailability_Empty(t *testing.T) {
	avail, total := CalculateAvailability(nil, 0.7)
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
		records[i] = &storage.ProbeRecord{Status: 1}
	}
	avail, total := CalculateAvailability(records, 0.7)
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
		records[i] = &storage.ProbeRecord{Status: 0}
	}
	avail, total := CalculateAvailability(records, 0.7)
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if avail != 0.0 {
		t.Errorf("expected availability=0.0, got %f", avail)
	}
}

func TestCalculateAvailability_Mixed(t *testing.T) {
	// 7 green + 3 yellow, degradedWeight=0.7
	// weighted = 7*1.0 + 3*0.7 = 7 + 2.1 = 9.1
	// availability = (9.1 / 10) * 100 = 91.0
	records := make([]*storage.ProbeRecord, 10)
	for i := 0; i < 7; i++ {
		records[i] = &storage.ProbeRecord{Status: 1}
	}
	for i := 7; i < 10; i++ {
		records[i] = &storage.ProbeRecord{Status: 2}
	}
	avail, total := CalculateAvailability(records, 0.7)
	if total != 10 {
		t.Errorf("expected total=10, got %d", total)
	}
	expected := 91.0
	if avail < expected-0.01 || avail > expected+0.01 {
		t.Errorf("expected availability≈%.1f, got %f", expected, avail)
	}
}

func TestCalculateAvailability_MixedWithRed(t *testing.T) {
	// 5 green + 3 yellow + 2 red, degradedWeight=0.5
	// weighted = 5*1.0 + 3*0.5 + 2*0.0 = 5 + 1.5 = 6.5
	// availability = (6.5 / 10) * 100 = 65.0
	records := []*storage.ProbeRecord{
		{Status: 1}, {Status: 1}, {Status: 1}, {Status: 1}, {Status: 1},
		{Status: 2}, {Status: 2}, {Status: 2},
		{Status: 0}, {Status: 0},
	}
	avail, _ := CalculateAvailability(records, 0.5)
	expected := 65.0
	if avail < expected-0.01 || avail > expected+0.01 {
		t.Errorf("expected availability≈%.1f, got %f", expected, avail)
	}
}
