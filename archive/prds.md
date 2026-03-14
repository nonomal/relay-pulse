
# LLM Service Monitor Backend (With Hot Reload)

> **状态**: 历史参考（已迁移至 `archive/`），内容不再更新，仅供回溯早期 PRD。

## 1. 项目说明

这是一个基于 Go 的后端服务，用于监控多个 LLM 渠道的可用性。
**核心特性**：
*   **配置驱动**：所有服务商信息通过 YAML 定义。
*   **热更新**：修改 `config.yaml` 后，服务会自动重载配置，无需重启。
*   **实时监控**：后台定时任务并发检测接口连通性。
*   **历史回溯**：API 返回混合数据（实时状态 + 模拟的历史 GitHub 风格时间轴）。

## 2. 项目依赖 (`go.mod`)

请在项目根目录初始化模块并安装依赖：

```bash
# 1. 初始化
go mod init monitor

# 2. 安装依赖
# gin: Web框架
# cors: 跨域支持
# yaml: 解析配置文件
# fsnotify: 监听文件变化实现热更新
go get -u github.com/gin-gonic/gin
go get -u github.com/gin-contrib/cors
go get -u gopkg.in/yaml.v3
go get -u github.com/fsnotify/fsnotify
```

---

## 3. 配置文件 (`config.yaml`)

在项目根目录新建 `config.yaml`。支持 `{{API_KEY}}` 占位符自动替换。

```yaml
monitors:
  # --- 88code ---
  - provider: "88code"
    service: "cc"
    url: "https://api.88code.com/v1/chat/completions"
    method: "POST"
    api_key: "sk-xxxxxxxx"  # 修改为你的真实Key
    headers:
      Authorization: "Bearer {{API_KEY}}"
      Content-Type: "application/json"
    body: |
      {
        "model": "claude-3-opus",
        "messages": [{"role": "user", "content": "hi"}],
        "max_tokens": 1
      }

  - provider: "88code"
    service: "cx"
    url: "https://api.88code.com/v1/chat/completions"
    method: "POST"
    api_key: "sk-xxxxxxxx"
    headers:
      Authorization: "Bearer {{API_KEY}}"
      Content-Type: "application/json"
    body: |
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "hi"}],
        "max_tokens": 1
      }

  # --- DuckCoding (演示 Header 差异) ---
  - provider: "duckcoding"
    service: "cc"
    url: "https://api.duckcoding.com/v1/messages"
    method: "POST"
    api_key: "sk-duck-xxxx"
    headers:
      x-api-key: "{{API_KEY}}"
      anthropic-version: "2023-06-01"
      content-type: "application/json"
    body: |
      {
        "model": "claude-3-sonnet",
        "max_tokens": 1,
        "messages": [{"role": "user", "content": "hi"}]
      }
```

---

## 4. 完整代码 (`main.go`)

新建 `main.go`，将以下内容完整复制。

```go
package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// ================= 1. 数据结构定义 =================

// ServiceConfig 单个服务配置 (对应 YAML)
type ServiceConfig struct {
	Provider string            `yaml:"provider" json:"provider"`
	Service  string            `yaml:"service" json:"service"`
	URL      string            `yaml:"url" json:"url"`
	Method   string            `yaml:"method" json:"method"`
	Headers  map[string]string `yaml:"headers" json:"headers"`
	Body     string            `yaml:"body" json:"body"`
	APIKey   string            `yaml:"api_key" json:"-"` // 不返回给前端
}

// AppConfig 根配置
type AppConfig struct {
	Monitors []ServiceConfig `yaml:"monitors"`
}

// LatestStatus 实时检测结果
type LatestStatus struct {
	Status  int   // 1=绿, 0=红, 2=黄
	Latency int   // ms
	Time    int64 // 更新时间戳
}

// TimePoint 前端图表数据点
type TimePoint struct {
	Time    string `json:"time"`
	Status  int    `json:"status"`
	Latency int    `json:"latency"`
}

// MonitorResult API 返回结构
type MonitorResult struct {
	Provider string       `json:"provider"`
	Service  string       `json:"service"`
	Current  LatestStatus `json:"current_status"`
	Timeline []TimePoint  `json:"timeline"`
}

// GlobalState 全局状态管理 (包含配置和检测结果)
type GlobalState struct {
	sync.RWMutex
	Config      AppConfig
	StatusCache map[string]map[string]LatestStatus // [Provider][Service] -> Status
}

var state = &GlobalState{
	StatusCache: make(map[string]map[string]LatestStatus),
}

// ================= 2. 配置管理与热更新 =================

// loadConfig 读取并解析 YAML
func loadConfig(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var newConfig AppConfig
	if err := yaml.Unmarshal(data, &newConfig); err != nil {
		return err
	}

	// 线程安全更新配置
	state.Lock()
	state.Config = newConfig
	state.Unlock()

	log.Printf("[Config] 已加载 %d 个监控任务", len(newConfig.Monitors))
	return nil
}

// watchConfig 监听文件变化实现热更新
func watchConfig(filename string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// 监听写入或重命名事件
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Rename == fsnotify.Rename {
					log.Println("[Config] 检测到配置文件变更，正在重载...")
					// 稍微延迟一下，避免文件写入未完成
					time.Sleep(100 * time.Millisecond)
					if err := loadConfig(filename); err != nil {
						log.Printf("[Config] 重载失败 (保持旧配置): %v", err)
					} else {
						log.Println("[Config] 热更新成功！")
						// 触发一次立即巡检
						go runAllChecks()
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("[Config] Watch error: %v", err)
			}
		}
	}()

	err = watcher.Add(filename)
	if err != nil {
		log.Fatal(err)
	}
	<-done
}

// ================= 3. 核心探测逻辑 =================

func performCheck(cfg ServiceConfig) LatestStatus {
	// 1. 准备请求
	reqBody := bytes.NewBuffer([]byte(cfg.Body))
	req, err := http.NewRequest(cfg.Method, cfg.URL, reqBody)
	if err != nil {
		return LatestStatus{Status: 0, Latency: 0, Time: time.Now().Unix()}
	}

	// 2. 注入 Headers 和 API Key
	for k, v := range cfg.Headers {
		val := strings.ReplaceAll(v, "{{API_KEY}}", cfg.APIKey)
		req.Header.Set(k, val)
	}

	// 3. 发送请求 (10s 超时)
	client := &http.Client{Timeout: 10 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	latency := int(time.Since(start).Milliseconds())

	// 4. 判定结果
	if err != nil {
		log.Printf("[Probe] ERROR %s-%s: %v", cfg.Provider, cfg.Service, err)
		return LatestStatus{Status: 0, Latency: 0, Time: time.Now().Unix()}
	}
	defer resp.Body.Close()

	// 丢弃Body数据，只读取少量以完成连接
	io.CopyN(io.Discard, resp.Body, 1024)

	status := 1 // Green
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		status = 1
	} else if resp.StatusCode >= 500 || resp.StatusCode == 429 {
		status = 2 // Yellow
	} else {
		status = 0 // Red (401, 403, 404 etc)
	}

	if latency > 5000 && status == 1 {
		status = 2 // Latency too high
	}

	log.Printf("[Probe] %s-%s | Code: %d | Latency: %dms | Status: %d",
		cfg.Provider, cfg.Service, resp.StatusCode, latency, status)

	return LatestStatus{Status: status, Latency: latency, Time: time.Now().Unix()}
}

func runAllChecks() {
	// 1. 获取当前配置快照 (避免遍历时配置变更)
	state.RLock()
	tasks := state.Config.Monitors
	state.RUnlock()

	if len(tasks) == 0 {
		return
	}

	var wg sync.WaitGroup
	// 限制并发数防止把本机跑挂，虽然 Goroutine 很轻
	sem := make(chan struct{}, 10)

	for _, task := range tasks {
		wg.Add(1)
		go func(t ServiceConfig) {
			defer wg.Done()
			sem <- struct{}{} // 获取信号量
			res := performCheck(t)
			<-sem // 释放

			// 写入结果
			state.Lock()
			if state.StatusCache[t.Provider] == nil {
				state.StatusCache[t.Provider] = make(map[string]LatestStatus)
			}
			state.StatusCache[t.Provider][t.Service] = res
			state.Unlock()
		}(task)
	}
	wg.Wait()
}

func startScheduler() {
	ticker := time.NewTicker(1 * time.Minute)

	// 启动时立即跑一次
	go runAllChecks()

	go func() {
		for range ticker.C {
			runAllChecks()
		}
	}()
}

// ================= 4. 辅助与 API =================

// 生成模拟历史数据，但强制最后一个点为真实状态
func generateMockTimeline(period string, current LatestStatus) []TimePoint {
	points := make([]TimePoint, 0)

	count := 24
	step := time.Hour
	format := "15:04"

	if period == "7d" {
		count = 7
		step = 24 * time.Hour
		format = "2006-01-02"
	} else if period == "30d" {
		count = 30
		step = 24 * time.Hour
		format = "2006-01-02"
	}

	now := time.Now()
	for i := count - 1; i >= 0; i-- {
		t := now.Add(-time.Duration(i) * step)

		// 默认逻辑
		s := 1
		l := rand.Intn(200) + 50

		// 如果是当前时间点 (最后一个)，使用真实数据
		if i == 0 {
			if current.Time > 0 { // 只有当有真实检测数据时才覆盖
				s = current.Status
				l = current.Latency
			}
		} else {
			// 模拟随机波动
			r := rand.Intn(100)
			if r > 95 { s = 0; l = 0 } else if r > 85 { s = 2; l = 800 }
		}

		points = append(points, TimePoint{
			Time:    t.Format(format),
			Status:  s,
			Latency: l,
		})
	}
	return points
}

func main() {
	configFile := "config.yaml"

	// 1. 初始加载配置
	if err := loadConfig(configFile); err != nil {
		log.Fatalf("无法加载配置文件: %v", err)
	}

	// 2. 启动配置监听 (热更新)
	go watchConfig(configFile)

	// 3. 启动定时巡检
	startScheduler()

	// 4. 启动 Web 服务
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.Default())

	r.GET("/api/status", func(c *gin.Context) {
		period := c.DefaultQuery("period", "24h")
		qProvider := c.DefaultQuery("provider", "all")
		qService := c.DefaultQuery("service", "all")

		var response []MonitorResult

		// 读取状态和配置
		state.RLock()
		currentConfig := state.Config.Monitors
		// 复制一份 map 防止并发读写冲突
		// 注意：这里只是简单的读取，为了高性能，深拷贝视情况而定。
		// 简单场景下，直接读锁保护内层读取即可。
		state.RUnlock()

		// 临时去重 map
		seen := make(map[string]bool)

		for _, task := range currentConfig {
			key := task.Provider + "-" + task.Service
			if seen[key] { continue }

			// 筛选
			if qProvider != "all" && qProvider != task.Provider { continue }
			if qService != "all" && qService != task.Service { continue }

			// 获取实时状态
			var current LatestStatus
			state.RLock() // 再次加读锁读取 map 内容
			if pMap, ok := state.StatusCache[task.Provider]; ok {
				if s, ok := pMap[task.Service]; ok {
					current = s
				}
			}
			state.RUnlock()

			// 生成 Timeline
			timeline := generateMockTimeline(period, current)

			response = append(response, MonitorResult{
				Provider: task.Provider,
				Service:  task.Service,
				Current:  current,
				Timeline: timeline,
			})
			seen[key] = true
		}

		c.JSON(http.StatusOK, gin.H{
			"meta": gin.H{
				"period": period,
				"count": len(response),
			},
			"data": response,
		})
	})

	port := "8080"
	fmt.Printf("\n🚀 监控服务已启动\n👉 API 地址: http://localhost:%s/api/status\n👉 配置文件: %s (支持热更新)\n\n", port, configFile)

	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
```

## 5. 验证热更新

1.  **运行程序**: `go run main.go`
2.  **修改配置**: 用编辑器打开 `config.yaml`，比如把 `88code` 改名为 `88code_NEW`，或者修改某个 API Key。
3.  **保存文件**: 保存 `config.yaml`。
4.  **观察终端**: 你会在终端看到类似 `[Config] 检测到配置文件变更，正在重载...` 的日志。
5.  **刷新 API**: 再次访问 `http://localhost:8080/api/status`，你会发现返回的数据已经变成了新的配置，且后台巡检任务也自动切换到了新的目标。
