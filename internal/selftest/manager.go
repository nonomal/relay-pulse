package selftest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"monitor/internal/logger"
)

// TestJobManager manages the lifecycle of self-test jobs
type TestJobManager struct {
	mu      sync.RWMutex
	jobs    map[string]*TestJob // id -> job
	queue   []*TestJob          // waiting queue (FIFO)
	running map[string]*TestJob // currently running jobs

	maxConcurrent int           // Maximum concurrent jobs (default 10)
	maxQueueSize  int           // Maximum queue length (default 50)
	jobTimeout    time.Duration // Job timeout (default 30s)
	resultTTL     time.Duration // Result retention time (2 minutes)

	prober    *SelfTestProber // 自助测试专用探测器（安全 HTTP 客户端）
	limiter   *IPLimiter      // IP rate limiter
	ssrfGuard *SSRFGuard      // SSRF protection

	stopCleanup chan struct{}  // Signal to stop cleanup goroutine
	stopOnce    sync.Once      // Ensure Stop is called only once
	wg          sync.WaitGroup // Wait group for graceful shutdown
}

// NewTestJobManager creates a new test job manager
func NewTestJobManager(
	maxConcurrent int,
	maxQueueSize int,
	jobTimeout time.Duration,
	resultTTL time.Duration,
	rateLimitPerMinute int,
) *TestJobManager {
	mgr := &TestJobManager{
		jobs:          make(map[string]*TestJob),
		queue:         make([]*TestJob, 0),
		running:       make(map[string]*TestJob),
		maxConcurrent: maxConcurrent,
		maxQueueSize:  maxQueueSize,
		jobTimeout:    jobTimeout,
		resultTTL:     resultTTL,
		limiter:       NewIPLimiter(rateLimitPerMinute, rateLimitPerMinute),
		ssrfGuard:     NewSSRFGuard(),
		stopCleanup:   make(chan struct{}),
	}
	// 创建自助测试专用探测器（使用安全 HTTP 客户端）
	mgr.prober = NewSelfTestProber(mgr.ssrfGuard, DefaultMaxResponseBytes)

	// Start cleanup worker
	mgr.wg.Add(1)
	go mgr.cleanupWorker()

	return mgr
}

// CreateJob creates a new test job and enqueues it
// Returns the job ID and any error
func (m *TestJobManager) CreateJob(
	testType, apiURL, apiKey string,
) (*TestJob, error) {
	// 1. SSRF protection
	if err := m.ssrfGuard.ValidateURL(apiURL); err != nil {
		return nil, &Error{
			Code:    ErrCodeInvalidURL,
			Message: "API 地址不安全或不合法",
			Err:     err,
		}
	}

	// 2. Test type validation
	testTypeDef, ok := GetTestType(testType)
	if !ok {
		return nil, &Error{
			Code:    ErrCodeUnknownTestType,
			Message: "不支持的测试类型",
			Err:     fmt.Errorf("unknown test type: %s", testType),
		}
	}

	// 4. Check queue capacity
	m.mu.Lock()
	if len(m.queue) >= m.maxQueueSize {
		m.mu.Unlock()
		return nil, &Error{
			Code:    ErrCodeQueueFull,
			Message: "队列已满，请稍后再试",
			Err:     fmt.Errorf("queue is full (max: %d)", m.maxQueueSize),
		}
	}

	// 5. Create job
	job := &TestJob{
		ID:        uuid.New().String(),
		TestType:  testType,
		APIURL:    apiURL,
		APIKey:    apiKey, // Stored in memory only, never serialized
		Status:    StatusQueued,
		QueuePos:  len(m.queue) + 1,
		CreatedAt: time.Now(),
	}

	// 6. Enqueue
	m.queue = append(m.queue, job)
	m.jobs[job.ID] = job
	m.mu.Unlock()

	logger.Info("selftest", "Job created and queued",
		"job_id", job.ID, "test_type", testType, "queue_position", job.QueuePos)

	// 7. Try to schedule immediately
	m.scheduleNext()

	// Store test type definition for later use
	_ = testTypeDef // We'll use this in worker

	// 8. Return snapshot to avoid data race with worker
	// Worker may have already started modifying job fields after scheduleNext()
	return job.Snapshot(), nil
}

// GetJob retrieves a job by ID and returns a snapshot (copy) to avoid data races
func (m *TestJobManager) GetJob(id string) (*TestJob, error) {
	m.mu.RLock()
	job, ok := m.jobs[id]
	m.mu.RUnlock()

	if !ok {
		return nil, &Error{
			Code:    ErrCodeJobNotFound,
			Message: "任务不存在或已过期",
			Err:     fmt.Errorf("job not found: %s", id),
		}
	}

	return job.Snapshot(), nil
}

// scheduleNext attempts to schedule the next job from the queue
func (m *TestJobManager) scheduleNext() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we have capacity and jobs in queue
	if len(m.running) >= m.maxConcurrent || len(m.queue) == 0 {
		return
	}

	// Dequeue first job
	job := m.queue[0]
	m.queue = m.queue[1:]

	// Update queue positions for remaining jobs
	for i, queuedJob := range m.queue {
		queuedJob.mu.Lock()
		queuedJob.QueuePos = i + 1
		queuedJob.mu.Unlock()
	}

	// Mark as running
	m.running[job.ID] = job
	now := time.Now()
	job.setRunning(now)

	logger.Info("selftest", "Job started",
		"job_id", job.ID, "test_type", job.TestType, "running_count", len(m.running))

	// Start worker in background
	m.wg.Add(1)
	go m.worker(job)
}

// worker executes a test job
func (m *TestJobManager) worker(job *TestJob) {
	defer func() {
		// Release semaphore and try to schedule next job
		m.mu.Lock()
		delete(m.running, job.ID)
		m.mu.Unlock()

		m.scheduleNext()
		m.wg.Done()
	}()

	// Get test type definition (读取 job 字段需要加锁)
	job.mu.RLock()
	testType := job.TestType
	apiURL := job.APIURL
	apiKey := job.APIKey
	job.mu.RUnlock()

	testTypeDef, ok := GetTestType(testType)
	if !ok {
		now := time.Now()
		job.mu.Lock()
		job.Status = StatusFailed
		job.ErrorMessage = fmt.Sprintf("unknown test type: %s", testType)
		job.FinishedAt = &now
		job.mu.Unlock()
		return
	}

	// Build probe configuration
	cfg, err := testTypeDef.Builder.Build(apiURL, apiKey)
	if err != nil {
		now := time.Now()
		job.mu.Lock()
		job.Status = StatusFailed
		job.ErrorMessage = fmt.Sprintf("failed to build config: %v", err)
		job.FinishedAt = &now
		job.mu.Unlock()
		return
	}

	// Execute probe with timeout
	ctx, cancel := context.WithTimeout(context.Background(), m.jobTimeout)
	defer cancel()

	result := m.prober.Probe(ctx, cfg)

	// 写入结果（持 job 独立锁，避免 data race）
	now := time.Now()
	job.mu.Lock()
	job.ProbeStatus = result.Status
	job.SubStatus = result.SubStatus
	job.HTTPCode = result.HTTPCode
	job.Latency = result.Latency
	job.ResponseSnippet = result.ResponseSnippet
	if result.Err != nil {
		job.ErrorMessage = result.Err.Error()
	}

	// Determine final status
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		job.Status = StatusTimeout
	} else if result.Status == 0 {
		job.Status = StatusFailed
	} else {
		job.Status = StatusSuccess
	}
	job.FinishedAt = &now
	job.mu.Unlock()

	logger.Info("selftest", "Job completed",
		"job_id", job.ID,
		"status", job.Status,
		"probe_status", result.Status,
		"latency", result.Latency,
		"http_code", result.HTTPCode)
}

// cleanupWorker periodically removes expired jobs
func (m *TestJobManager) cleanupWorker() {
	defer m.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.stopCleanup:
			return
		}
	}
}

// cleanup removes jobs that exceeded TTL after completion
func (m *TestJobManager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, job := range m.jobs {
		// Only clean up terminal jobs that exceeded TTL (需要加锁读取 FinishedAt)
		job.mu.RLock()
		finishedAt := job.FinishedAt
		job.mu.RUnlock()

		if finishedAt != nil && now.Sub(*finishedAt) > m.resultTTL {
			delete(m.jobs, id)
			logger.Info("selftest", "Job cleaned up",
				"job_id", id, "finished_at", finishedAt)
		}
	}
}

// Stop gracefully stops the manager (idempotent, safe to call multiple times)
func (m *TestJobManager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCleanup)
		m.wg.Wait()
		m.limiter.Stop()
	})
}

// CheckRateLimit checks if the IP is allowed to make a request
func (m *TestJobManager) CheckRateLimit(ip string) bool {
	return m.limiter.Allow(ip)
}
