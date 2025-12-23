package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"notifier/internal/config"
	"notifier/internal/storage"
)

// Event 状态变更事件（来自 relay-pulse API）
type Event struct {
	ID         int64  `json:"id"`
	Provider   string `json:"provider"`
	Service    string `json:"service"`
	Channel    string `json:"channel"`
	OldStatus  int    `json:"old_status"`
	NewStatus  int    `json:"new_status"`
	OldLatency int64  `json:"old_latency"`
	NewLatency int64  `json:"new_latency"`
	Timestamp  int64  `json:"timestamp"`
}

// EventsResponse /api/events 响应
type EventsResponse struct {
	Events []Event `json:"events"`
	Meta   struct {
		Count   int   `json:"count"`
		SinceID int64 `json:"since_id"`
	} `json:"meta"`
}

// EventHandler 事件处理回调
type EventHandler func(ctx context.Context, event *Event) error

// Poller 事件轮询器
type Poller struct {
	cfg        *config.Config
	storage    storage.Storage
	httpClient *http.Client
	handler    EventHandler

	mu       sync.Mutex
	running  bool
	stopChan chan struct{}
}

// NewPoller 创建轮询器
func NewPoller(cfg *config.Config, store storage.Storage, handler EventHandler) *Poller {
	return &Poller{
		cfg:     cfg,
		storage: store,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		handler:  handler,
		stopChan: make(chan struct{}),
	}
}

// Start 启动轮询
func (p *Poller) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("轮询器已在运行")
	}
	p.running = true
	p.stopChan = make(chan struct{})
	p.mu.Unlock()

	slog.Info("事件轮询器启动",
		"events_url", p.cfg.RelayPulse.EventsURL,
		"poll_interval", p.cfg.RelayPulse.PollInterval,
	)

	ticker := time.NewTicker(p.cfg.RelayPulse.PollInterval)
	defer ticker.Stop()

	// 立即执行一次
	p.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("轮询器收到停止信号")
			return ctx.Err()
		case <-p.stopChan:
			slog.Info("轮询器停止")
			return nil
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

// Stop 停止轮询
func (p *Poller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		close(p.stopChan)
		p.running = false
	}
}

// poll 执行一次轮询
func (p *Poller) poll(ctx context.Context) {
	// 获取游标
	cursor, err := p.storage.GetCursor(ctx)
	if err != nil {
		slog.Error("获取游标失败", "error", err)
		return
	}

	// 获取事件
	events, err := p.fetchEvents(ctx, cursor)
	if err != nil {
		slog.Warn("获取事件失败", "error", err)
		return
	}

	if len(events) == 0 {
		return
	}

	slog.Debug("获取到新事件", "count", len(events), "since_id", cursor)

	// 处理事件
	var maxID int64 = cursor
	for _, event := range events {
		if err := p.handler(ctx, &event); err != nil {
			slog.Error("处理事件失败", "event_id", event.ID, "error", err)
			continue
		}

		if event.ID > maxID {
			maxID = event.ID
		}
	}

	// 更新游标
	if maxID > cursor {
		if err := p.storage.UpdateCursor(ctx, maxID); err != nil {
			slog.Error("更新游标失败", "error", err)
		}
	}
}

// fetchEvents 从 relay-pulse 获取事件
func (p *Poller) fetchEvents(ctx context.Context, sinceID int64) ([]Event, error) {
	url := p.cfg.RelayPulse.EventsURL + "?since_id=" + strconv.FormatInt(sinceID, 10)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 添加 API Token（如果配置了）
	if p.cfg.RelayPulse.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.cfg.RelayPulse.APIToken)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("API Token 无效或缺失")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API 返回错误 [%d]: %s", resp.StatusCode, string(body))
	}

	var eventsResp EventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&eventsResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return eventsResp.Events, nil
}
