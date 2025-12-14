package selftest

import (
	"context"
	"sync"
	"time"
)

// JobStatus represents the current state of a test job
type JobStatus string

const (
	// StatusQueued indicates the job is waiting in queue
	StatusQueued JobStatus = "queued"
	// StatusRunning indicates the job is currently being executed
	StatusRunning JobStatus = "running"
	// StatusSuccess indicates the job completed successfully
	StatusSuccess JobStatus = "success"
	// StatusFailed indicates the job failed
	StatusFailed JobStatus = "failed"
	// StatusTimeout indicates the job exceeded the timeout limit
	StatusTimeout JobStatus = "timeout"
	// StatusCanceled indicates the job was canceled
	StatusCanceled JobStatus = "canceled"
)

// TestJob represents a single self-test task
type TestJob struct {
	// mu 保护任务字段并发读写（避免 data race）
	mu sync.RWMutex `json:"-"`

	// Basic identifiers
	ID       string    `json:"id"`        // UUID
	TestType string    `json:"test_type"` // "cc", "cx", etc.
	APIURL   string    `json:"api_url"`
	APIKey   string    `json:"-"` // Never serialize to JSON for security
	Status   JobStatus `json:"status"`
	QueuePos int       `json:"queue_position,omitempty"` // Only valid when Status == StatusQueued

	// Result fields (populated after completion)
	ProbeStatus     int    `json:"probe_status,omitempty"`     // 1/0/2 (green/red/yellow)
	SubStatus       string `json:"sub_status,omitempty"`       // Fine-grained status code
	HTTPCode        int    `json:"http_code,omitempty"`        // HTTP status code
	Latency         int    `json:"latency,omitempty"`          // Latency in milliseconds
	ErrorMessage    string `json:"error_message,omitempty"`    // Error description if any
	ResponseSnippet string `json:"response_snippet,omitempty"` // Server response snippet for debugging

	// Timestamps
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	// Internal fields (not serialized)
	ctx    context.Context    `json:"-"`
	cancel context.CancelFunc `json:"-"`
}

// Snapshot 返回一个不包含敏感字段的只读快照（避免并发访问问题）
func (j *TestJob) Snapshot() *TestJob {
	j.mu.RLock()
	defer j.mu.RUnlock()

	return &TestJob{
		ID:              j.ID,
		TestType:        j.TestType,
		APIURL:          j.APIURL,
		APIKey:          "", // 永不对外暴露
		Status:          j.Status,
		QueuePos:        j.QueuePos,
		ProbeStatus:     j.ProbeStatus,
		SubStatus:       j.SubStatus,
		HTTPCode:        j.HTTPCode,
		Latency:         j.Latency,
		ErrorMessage:    j.ErrorMessage,
		ResponseSnippet: j.ResponseSnippet,
		CreatedAt:       j.CreatedAt,
		StartedAt:       j.StartedAt,
		FinishedAt:      j.FinishedAt,
	}
}

// IsTerminal returns true if the job is in a terminal state (completed/failed/timeout/canceled)
func (j *TestJob) IsTerminal() bool {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.isTerminalLocked()
}

func (j *TestJob) isTerminalLocked() bool {
	return j.Status == StatusSuccess ||
		j.Status == StatusFailed ||
		j.Status == StatusTimeout ||
		j.Status == StatusCanceled
}

// IsActive returns true if the job is queued or running
func (j *TestJob) IsActive() bool {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.Status == StatusQueued || j.Status == StatusRunning
}

// setRunning 标记任务为运行中状态（内部使用，已持锁）
func (j *TestJob) setRunning(start time.Time) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = StatusRunning
	j.QueuePos = 0
	j.StartedAt = &start
}
