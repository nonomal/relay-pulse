package announcements

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"monitor/internal/config"
	"monitor/internal/logger"
)

func init() {
	// 初始化随机数种子，确保多实例启动抖动不同
	rand.Seed(time.Now().UnixNano())
}

// Announcement 公告条目（用于 API 响应）
type Announcement struct {
	ID        string    `json:"id"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
	Author    string    `json:"author,omitempty"`
}

// Snapshot 公告快照（供 API handler 直接序列化）
type Snapshot struct {
	Enabled bool `json:"enabled"`

	Source struct {
		Provider       string `json:"provider"`
		Repo           string `json:"repo"`
		Category       string `json:"category"`
		DiscussionsURL string `json:"discussionsUrl"`
	} `json:"source"`

	Window struct {
		Hours   int       `json:"hours"`
		StartAt time.Time `json:"startAt"`
		EndAt   time.Time `json:"endAt"`
	} `json:"window"`

	Fetch struct {
		FetchedAt  time.Time `json:"fetchedAt"`
		Stale      bool      `json:"stale"`
		TTLSeconds int       `json:"ttlSeconds"`
	} `json:"fetch"`

	Latest  *Announcement  `json:"latest,omitempty"`
	Items   []Announcement `json:"items"`
	Version string         `json:"version"`

	APIMaxAge int `json:"apiMaxAge"`
}

// Service 公告服务：负责定时从 GitHub 拉取 Announcements 分类并缓存
type Service struct {
	cfg   config.AnnouncementsConfig
	ghCfg config.GitHubConfig
	fetch *Fetcher
	nowFn func() time.Time // 用于测试注入

	mu      sync.RWMutex
	cached  *Snapshot
	expires time.Time
	lastErr error

	// singleflight 防止并发刷新
	refreshMu      sync.Mutex
	refreshing     bool
	refreshWaiters []chan struct{}

	stopMu   sync.Mutex
	running  bool
	stopCh   chan struct{}
	stoppedC chan struct{}
}

// NewService 创建公告服务
func NewService(cfg config.AnnouncementsConfig, ghCfg config.GitHubConfig) (*Service, error) {
	fetcher, err := NewFetcher(ghCfg)
	if err != nil {
		return nil, fmt.Errorf("创建 announcements fetcher 失败: %w", err)
	}

	return &Service{
		cfg:      cfg,
		ghCfg:    ghCfg,
		fetch:    fetcher,
		nowFn:    time.Now,
		stopCh:   make(chan struct{}),
		stoppedC: make(chan struct{}),
	}, nil
}

// Start 启动后台轮询（使用 ctx 控制生命周期）
func (s *Service) Start(ctx context.Context) {
	s.stopMu.Lock()
	if s.running {
		s.stopMu.Unlock()
		return
	}
	s.running = true
	s.stopMu.Unlock()

	go func() {
		defer close(s.stoppedC)

		// 启动时随机抖动 0-30s，避免多实例同时打 GitHub
		jitter := time.Duration(rand.Int63n(int64(30 * time.Second)))
		logger.Info("announcements", "公告服务启动",
			"poll_interval", s.cfg.PollInterval,
			"window_hours", s.cfg.WindowHours,
			"startup_jitter", jitter.String())

		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-time.After(jitter):
		}

		// 首次刷新
		s.refreshOnce(ctx, true)

		ticker := time.NewTicker(s.cfg.PollIntervalDuration)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Info("announcements", "公告服务已停止（context 取消）")
				return
			case <-s.stopCh:
				logger.Info("announcements", "公告服务已停止")
				return
			case <-ticker.C:
				s.refreshOnce(ctx, false)
			}
		}
	}()
}

// Stop 停止后台轮询
func (s *Service) Stop() {
	s.stopMu.Lock()
	defer s.stopMu.Unlock()

	if !s.running {
		return
	}

	close(s.stopCh)
	<-s.stoppedC
	s.running = false
}

// GetAnnouncements 获取公告快照
func (s *Service) GetAnnouncements(ctx context.Context) (*Snapshot, error) {
	// 功能关闭时返回禁用状态
	if !s.cfg.IsEnabled() {
		return s.disabledSnapshot(), nil
	}

	now := s.nowFn()

	s.mu.RLock()
	cached := s.cached
	exp := s.expires
	s.mu.RUnlock()

	// 缓存有效
	if cached != nil && now.Before(exp) {
		return cached, nil
	}

	// 缓存过期：同步刷新一次（使用 singleflight）
	s.refreshOnce(ctx, true)

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.cached != nil {
		return s.cached, nil
	}

	if s.lastErr != nil {
		return nil, s.lastErr
	}

	return nil, fmt.Errorf("公告数据暂不可用")
}

// refreshOnce 执行一次刷新（带 singleflight 保护）
func (s *Service) refreshOnce(ctx context.Context, allowStale bool) {
	if !s.cfg.IsEnabled() {
		return
	}

	// singleflight: 检查是否已有刷新在进行
	s.refreshMu.Lock()
	if s.refreshing {
		// 已有刷新在进行，等待完成
		waiter := make(chan struct{})
		s.refreshWaiters = append(s.refreshWaiters, waiter)
		s.refreshMu.Unlock()

		select {
		case <-waiter:
			return
		case <-ctx.Done():
			return
		}
	}
	s.refreshing = true
	s.refreshMu.Unlock()

	// 执行实际刷新
	defer func() {
		s.refreshMu.Lock()
		s.refreshing = false
		// 通知所有等待者
		for _, w := range s.refreshWaiters {
			close(w)
		}
		s.refreshWaiters = nil
		s.refreshMu.Unlock()
	}()

	s.doRefresh(ctx, allowStale)
}

// doRefresh 实际执行刷新逻辑
func (s *Service) doRefresh(ctx context.Context, allowStale bool) {
	now := s.nowFn()
	windowStart := now.Add(-s.cfg.WindowDuration)

	// 获取分类 ID
	categoryID, err := s.fetch.FetchCategoryID(ctx, s.cfg.Owner, s.cfg.Repo, s.cfg.CategoryName)
	if err != nil {
		s.onRefreshError(err, allowStale)
		return
	}

	// 获取讨论列表
	discussions, err := s.fetch.FetchDiscussionsByCategoryID(ctx, s.cfg.Owner, s.cfg.Repo, categoryID, s.cfg.MaxItems)
	if err != nil {
		s.onRefreshError(err, allowStale)
		return
	}

	// 过滤窗口内的讨论
	items := make([]Announcement, 0, len(discussions))
	for _, d := range discussions {
		if d.CreatedAt.Before(windowStart) {
			continue
		}
		items = append(items, Announcement{
			ID:        d.ID,
			Number:    d.Number,
			Title:     d.Title,
			URL:       d.URL,
			CreatedAt: d.CreatedAt,
			Author:    d.AuthorLogin,
		})
	}

	// 构建快照
	snap := s.buildSnapshot(now, windowStart, items)

	s.mu.Lock()
	s.cached = snap
	s.expires = now.Add(s.cfg.PollIntervalDuration)
	s.lastErr = nil
	s.mu.Unlock()

	logger.Debug("announcements", "刷新公告成功", "count", len(items))
}

// onRefreshError 处理刷新错误
func (s *Service) onRefreshError(err error, allowStale bool) {
	logger.Warn("announcements", "刷新公告失败", "error", err)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastErr = err

	// 允许使用旧数据：创建新的 snapshot 副本并标记 stale
	// 不直接修改原 cached，避免并发读取时的数据竞争
	if allowStale && s.cached != nil {
		staleCopy := s.copySnapshotWithStale(s.cached)
		s.cached = staleCopy
		s.expires = s.nowFn().Add(1 * time.Minute)
	}
}

// copySnapshotWithStale 创建 snapshot 的副本并标记为 stale
func (s *Service) copySnapshotWithStale(src *Snapshot) *Snapshot {
	if src == nil {
		return nil
	}

	// 创建新的 snapshot 副本
	dst := &Snapshot{
		Enabled:   src.Enabled,
		Version:   src.Version,
		APIMaxAge: src.APIMaxAge,
	}

	dst.Source = src.Source
	dst.Window = src.Window
	dst.Fetch = src.Fetch
	dst.Fetch.Stale = true // 标记为 stale

	// 复制 items（切片是引用类型，但这里不需要深拷贝因为 items 不会被修改）
	dst.Items = src.Items
	dst.Latest = src.Latest

	return dst
}

// buildSnapshot 构建公告快照
func (s *Service) buildSnapshot(now, windowStart time.Time, items []Announcement) *Snapshot {
	snap := &Snapshot{
		Enabled:   true,
		Items:     items,
		APIMaxAge: s.cfg.APIMaxAge,
	}

	snap.Source.Provider = "github"
	snap.Source.Repo = fmt.Sprintf("%s/%s", s.cfg.Owner, s.cfg.Repo)
	snap.Source.Category = s.cfg.CategoryName
	snap.Source.DiscussionsURL = fmt.Sprintf("https://github.com/%s/%s/discussions", s.cfg.Owner, s.cfg.Repo)

	snap.Window.Hours = s.cfg.WindowHours
	snap.Window.StartAt = windowStart
	snap.Window.EndAt = now

	snap.Fetch.FetchedAt = now
	snap.Fetch.Stale = false
	snap.Fetch.TTLSeconds = int(s.cfg.PollIntervalDuration.Seconds())

	if len(items) > 0 {
		snap.Latest = &items[0]
		snap.Version = fmt.Sprintf("%s#%d", items[0].CreatedAt.UTC().Format(time.RFC3339), items[0].Number)
	} else {
		snap.Version = ""
	}

	return snap
}

// disabledSnapshot 返回禁用状态的快照
func (s *Service) disabledSnapshot() *Snapshot {
	now := s.nowFn()
	return &Snapshot{
		Enabled:   false,
		Items:     []Announcement{},
		APIMaxAge: 3600, // 禁用时缓存 1 小时
		Source: struct {
			Provider       string `json:"provider"`
			Repo           string `json:"repo"`
			Category       string `json:"category"`
			DiscussionsURL string `json:"discussionsUrl"`
		}{
			Provider:       "github",
			Repo:           fmt.Sprintf("%s/%s", s.cfg.Owner, s.cfg.Repo),
			Category:       s.cfg.CategoryName,
			DiscussionsURL: fmt.Sprintf("https://github.com/%s/%s/discussions", s.cfg.Owner, s.cfg.Repo),
		},
		Window: struct {
			Hours   int       `json:"hours"`
			StartAt time.Time `json:"startAt"`
			EndAt   time.Time `json:"endAt"`
		}{
			Hours:   s.cfg.WindowHours,
			StartAt: now.Add(-s.cfg.WindowDuration),
			EndAt:   now,
		},
		Fetch: struct {
			FetchedAt  time.Time `json:"fetchedAt"`
			Stale      bool      `json:"stale"`
			TTLSeconds int       `json:"ttlSeconds"`
		}{
			FetchedAt:  time.Time{},
			Stale:      false,
			TTLSeconds: 3600,
		},
	}
}
