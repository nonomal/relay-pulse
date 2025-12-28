package scheduler

import (
	"container/heap"
	"context"
	"math/rand"
	"sync"
	"time"

	"monitor/internal/config"
	"monitor/internal/events"
	"monitor/internal/logger"
	"monitor/internal/monitor"
	"monitor/internal/storage"
)

// task 表示一个待调度的探测任务
type task struct {
	monitor  config.ServiceConfig // 监测配置
	interval time.Duration        // 该任务的巡检间隔
	nextRun  time.Time            // 下次执行时间
	index    int                  // 在堆中的索引（heap.Interface 需要）
}

// taskHeap 按下一次触发时间排序的最小堆
type taskHeap []*task

func (h taskHeap) Len() int           { return len(h) }
func (h taskHeap) Less(i, j int) bool { return h[i].nextRun.Before(h[j].nextRun) }
func (h taskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *taskHeap) Push(x any) {
	t := x.(*task)
	t.index = len(*h)
	*h = append(*h, t)
}

func (h *taskHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // 避免内存泄漏
	item.index = -1
	*h = old[:n-1]
	return item
}

// Scheduler 调度器（最小堆调度架构）
// 支持每个监测项独立的巡检间隔
type Scheduler struct {
	prober       *monitor.Prober
	eventService *events.Service // 事件服务（可选）

	mu      sync.Mutex
	running bool
	timer   *time.Timer   // 单一定时器，等待最近任务
	tasks   taskHeap      // 任务最小堆
	sem     chan struct{} // 并发控制信号量
	wakeCh  chan struct{} // 唤醒信号（配置变更时）
	ctx     context.Context
	cancel  context.CancelFunc

	// 配置引用（支持热更新）
	cfg      *config.AppConfig
	cfgMu    sync.RWMutex
	fallback time.Duration // 默认巡检间隔（创建时传入）
}

// NewScheduler 创建调度器
func NewScheduler(store storage.Storage, interval time.Duration) *Scheduler {
	return &Scheduler{
		prober:   monitor.NewProber(store),
		fallback: interval,
		wakeCh:   make(chan struct{}, 1),
	}
}

// SetEventService 设置事件服务
// 用于探测完成后检测状态变更并产生事件
func (s *Scheduler) SetEventService(svc *events.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventService = svc
}

// Start 启动调度器
func (s *Scheduler) Start(ctx context.Context, cfg *config.AppConfig) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	// 保存初始配置并初始化任务堆（启动时错峰）
	s.rebuildTasks(cfg, true)

	// 启动调度循环
	go s.loop()

	logger.Info("scheduler", "调度器已启动", "monitors", len(cfg.Monitors))
}

// UpdateConfig 更新配置（热更新时调用）
func (s *Scheduler) UpdateConfig(cfg *config.AppConfig) {
	s.rebuildTasks(cfg, false)
	logger.Info("scheduler", "配置已更新，调度任务已重建")
}

// TriggerNow 立即触发所有任务的巡检
func (s *Scheduler) TriggerNow() {
	s.mu.Lock()
	if !s.running || len(s.tasks) == 0 {
		s.mu.Unlock()
		return
	}

	// 将所有任务的 nextRun 设为当前时间
	now := time.Now()
	for _, t := range s.tasks {
		t.nextRun = now
	}
	heap.Init(&s.tasks)
	s.resetTimerLocked()
	s.notifyWakeLocked()
	s.mu.Unlock()

	logger.Info("scheduler", "已触发即时巡检")
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}

	// 停止定时器
	if s.timer != nil {
		if !s.timer.Stop() {
			select {
			case <-s.timer.C:
			default:
			}
		}
		s.timer = nil
	}

	s.running = false
	if s.cancel != nil {
		s.cancel()
	}
	// 唤醒 loop 以便退出
	s.notifyWakeLocked()
	s.mu.Unlock()

	s.prober.Close()
	logger.Info("scheduler", "调度器已停止")
}

// rebuildTasks 根据配置重建调度任务堆
// startup=true 时使用启动模式错峰（固定 2 秒间隔）
func (s *Scheduler) rebuildTasks(cfg *config.AppConfig, startup bool) {
	if cfg == nil {
		return
	}

	// 更新配置引用
	s.cfgMu.Lock()
	s.cfg = cfg
	s.cfgMu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	monitorCount := len(cfg.Monitors)
	if monitorCount == 0 {
		s.tasks = s.tasks[:0]
		s.resetTimerLocked()
		s.notifyWakeLocked() // 唤醒 loop 以便重新检查状态
		return
	}

	// 统计禁用和冷板的监测项数量
	disabledCount := 0
	coldCount := 0
	for _, m := range cfg.Monitors {
		if m.Disabled {
			disabledCount++
			continue
		}
		// 冷板项：启用 boards 后不创建探测任务，仅展示历史数据
		if cfg.Boards.Enabled && m.Board == "cold" {
			coldCount++
		}
	}
	activeCount := monitorCount - disabledCount - coldCount

	// 如果所有监测项都被禁用或冷板，清空任务
	if activeCount == 0 {
		s.tasks = s.tasks[:0]
		s.resetTimerLocked()
		s.notifyWakeLocked()
		logger.Info("scheduler", "所有监测项已禁用/冷板，调度器无任务",
			"total", monitorCount, "disabled", disabledCount, "cold", coldCount)
		return
	}

	// 并发控制：-1 表示与活跃监测数持平；>0 为硬上限
	maxConcurrency := cfg.MaxConcurrency
	if maxConcurrency == -1 {
		maxConcurrency = activeCount
	}
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}
	s.sem = make(chan struct{}, maxConcurrency)
	logger.Info("scheduler", "并发控制已更新",
		"max_concurrency", maxConcurrency, "total", monitorCount,
		"disabled", disabledCount, "active", activeCount)

	// 错峰策略计算（基于活跃监测数）
	useStagger := cfg.ShouldStaggerProbes() && activeCount > 1
	var baseDelay, jitterRange time.Duration

	if useStagger {
		if startup {
			// 启动模式：固定 2 秒间隔，避免瞬时压力
			baseDelay = 2 * time.Second
			jitterRange = 400 * time.Millisecond // ±20%
			logger.Info("scheduler", "启动模式：探测将以固定间隔错峰执行",
				"base_delay", baseDelay, "jitter", jitterRange)
		} else {
			// 热更新模式：基于最小 interval 计算错峰
			minInterval := s.findMinInterval(cfg)
			if minInterval > 0 {
				baseDelay = minInterval / time.Duration(activeCount)
				jitterRange = baseDelay / 5 // ±20%
				logger.Info("scheduler", "探测将错峰执行",
					"base_delay", baseDelay, "jitter", jitterRange)
			} else {
				useStagger = false
			}
		}
	}

	// 构建任务堆
	s.tasks = s.tasks[:0]
	heap.Init(&s.tasks)
	now := time.Now()

	activeIdx := 0 // 独立的活跃索引，用于错峰计算
	for _, m := range cfg.Monitors {
		// 跳过已禁用的监测项（不探测、不存储）
		if m.Disabled {
			continue
		}

		// 跳过冷板项：启用 boards 后不探测（仅展示历史数据）
		if cfg.Boards.Enabled && m.Board == "cold" {
			continue
		}

		// 使用监测项自己的 interval，为空则使用全局 fallback
		interval := m.IntervalDuration
		if interval == 0 {
			interval = s.fallback
		}

		// 计算首次执行时间（考虑错峰）
		nextRun := now
		if useStagger {
			delay := computeStaggerDelay(baseDelay, jitterRange, activeIdx)
			nextRun = now.Add(delay)
		}

		heap.Push(&s.tasks, &task{
			monitor:  m,
			interval: interval,
			nextRun:  nextRun,
		})
		activeIdx++
	}

	s.resetTimerLocked()
	s.notifyWakeLocked()
}

// findMinInterval 找到所有活跃监测项中最小的 interval（跳过已禁用和冷板的）
func (s *Scheduler) findMinInterval(cfg *config.AppConfig) time.Duration {
	minInterval := cfg.IntervalDuration
	for _, m := range cfg.Monitors {
		// 跳过已禁用的监测项
		if m.Disabled {
			continue
		}
		// 跳过冷板项：启用 boards 后不参与调度
		if cfg.Boards.Enabled && m.Board == "cold" {
			continue
		}
		if m.IntervalDuration > 0 && (minInterval == 0 || m.IntervalDuration < minInterval) {
			minInterval = m.IntervalDuration
		}
	}
	return minInterval
}

// loop 调度主循环
func (s *Scheduler) loop() {
	for {
		s.mu.Lock()
		running := s.running
		timer := s.timer
		ctx := s.ctx
		s.mu.Unlock()

		if !running {
			return
		}

		var timerC <-chan time.Time
		if timer != nil {
			timerC = timer.C
		}

		select {
		case <-ctx.Done():
			s.Stop()
			return

		case <-timerC:
			// 定时器触发，执行到期任务
			s.dispatchDue()

		case <-s.wakeCh:
			// 配置变更唤醒，重新计算等待时间
			// 循环继续，会重新获取 timer
		}
	}
}

// dispatchDue 执行所有已到期的任务
func (s *Scheduler) dispatchDue() {
	for {
		s.mu.Lock()
		if len(s.tasks) == 0 {
			s.resetTimerLocked()
			s.mu.Unlock()
			return
		}

		// 检查堆顶任务是否到期
		next := s.tasks[0]
		now := time.Now()
		if next.nextRun.After(now) {
			// 最近任务未到期，重置定时器等待
			s.resetTimerLocked()
			s.mu.Unlock()
			return
		}

		// 弹出到期任务
		heap.Pop(&s.tasks)
		s.mu.Unlock()

		// 异步执行探测任务
		s.runTask(next)

		// 使用"至少间隔"语义：下次执行时间 = max(计划时间+interval, 当前时间+interval)
		// 避免探测耗时超过 interval 时快速补跑多个周期
		plannedNext := next.nextRun.Add(next.interval)
		minNext := time.Now().Add(next.interval)
		if plannedNext.Before(minNext) {
			next.nextRun = minNext
		} else {
			next.nextRun = plannedNext
		}

		// 重新入队
		s.mu.Lock()
		heap.Push(&s.tasks, next)
		s.resetTimerLocked()
		s.mu.Unlock()
	}
}

// runTask 在并发控制下执行单个探测任务
func (s *Scheduler) runTask(t *task) {
	s.mu.Lock()
	ctx := s.ctx
	sem := s.sem
	eventSvc := s.eventService
	s.mu.Unlock()

	if ctx == nil || sem == nil {
		return
	}

	// 获取信号量
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return
	}

	// 异步执行，释放信号量
	go func(m config.ServiceConfig) {
		defer func() { <-sem }()

		result := s.prober.Probe(ctx, &m)
		record, err := s.prober.SaveResult(result)
		if err != nil {
			logger.Error("scheduler", "保存结果失败",
				"provider", m.Provider, "service", m.Service, "channel", m.Channel, "error", err)
			return
		}

		// 事件检测（如果启用）
		if eventSvc != nil && eventSvc.IsEnabled() {
			if event, err := eventSvc.ProcessRecord(record); err != nil {
				logger.Error("scheduler", "事件检测失败",
					"provider", m.Provider, "service", m.Service, "channel", m.Channel, "error", err)
			} else if event != nil {
				logger.Info("scheduler", "检测到状态变更",
					"provider", m.Provider, "service", m.Service, "channel", m.Channel,
					"event_type", event.EventType, "from", event.FromStatus, "to", event.ToStatus)
			}
		}
	}(t.monitor)
}

// resetTimerLocked 重置定时器到下一个任务（需持有 s.mu）
func (s *Scheduler) resetTimerLocked() {
	if len(s.tasks) == 0 {
		// 无任务，停止定时器
		if s.timer != nil {
			if !s.timer.Stop() {
				select {
				case <-s.timer.C:
				default:
				}
			}
			s.timer = nil
		}
		return
	}

	// 计算等待时间
	wait := max(time.Until(s.tasks[0].nextRun), 0)

	if s.timer == nil {
		s.timer = time.NewTimer(wait)
		return
	}

	// 重置现有定时器
	if !s.timer.Stop() {
		select {
		case <-s.timer.C:
		default:
		}
	}
	s.timer.Reset(wait)
}

// notifyWakeLocked 唤醒调度循环（需持有 s.mu）
func (s *Scheduler) notifyWakeLocked() {
	select {
	case s.wakeCh <- struct{}{}:
	default:
		// 已有唤醒信号，无需重复发送
	}
}

// computeStaggerDelay 计算错峰延迟时间
// 基准延迟 + 随机抖动（±20%）
// 注意：使用全局 rand（Go 1.20+ 并发安全）
func computeStaggerDelay(baseDelay, jitterRange time.Duration, index int) time.Duration {
	delay := baseDelay * time.Duration(index)
	if jitterRange <= 0 {
		if delay < 0 {
			return 0
		}
		return delay
	}

	max := int64(jitterRange)
	if max <= 0 {
		if delay < 0 {
			return 0
		}
		return delay
	}

	// 随机抖动：±jitterRange（使用全局 rand，Go 1.20+ 并发安全）
	offset := rand.Int63n(max*2+1) - max
	delay += time.Duration(offset)
	if delay < 0 {
		return 0
	}
	return delay
}
