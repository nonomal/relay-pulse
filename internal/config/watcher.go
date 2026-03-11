package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"monitor/internal/logger"
)

// Watcher 配置文件监听器
type Watcher struct {
	loader       *Loader
	filename     string
	watcher      *fsnotify.Watcher
	onReload     func(*AppConfig)
	debounceTime time.Duration
	watchMu      sync.Mutex
	watchedDirs  map[string]struct{}
}

// NewWatcher 创建配置监听器
func NewWatcher(loader *Loader, filename string, onReload func(*AppConfig)) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		loader:       loader,
		filename:     filename,
		watcher:      watcher,
		onReload:     onReload,
		debounceTime: 200 * time.Millisecond, // 防抖延迟
	}, nil
}

// Start 启动监听（监听父目录以兼容不同编辑器）
func (w *Watcher) Start(ctx context.Context) error {
	// 监听父目录而非文件本身，避免编辑器 rename 导致监听失效
	dir := filepath.Dir(w.filename)
	targetFile := filepath.Clean(w.filename) // 归一化配置文件路径
	if err := w.addWatch(dir); err != nil {
		return err
	}

	// templates 目录（用于 body include JSON）
	templatesDir := filepath.Clean(filepath.Join(dir, "templates"))
	templatesDirPrefix := templatesDir + string(filepath.Separator) // 预计算前缀
	if info, err := os.Stat(templatesDir); err == nil && info.IsDir() {
		if err := w.addWatch(templatesDir); err != nil {
			return err
		}
	}

	logger.Info("config", "开始监听配置文件", "file", w.filename, "dir", dir)

	go func() {
		var debounceTimer *time.Timer
		for {
			select {
			case <-ctx.Done():
				logger.Info("config", "配置监听器已停止")
				w.watcher.Close()
				return

			case event, ok := <-w.watcher.Events:
				if !ok {
					return
				}

				// 只关心目标配置文件和 templates/ 目录下 JSON 的写入/创建/重命名事件
				eventPath := filepath.Clean(event.Name) // 归一化事件路径
				isConfigFile := eventPath == targetFile
				isTemplateFile := strings.HasPrefix(eventPath, templatesDirPrefix)
				if !isConfigFile && !isTemplateFile {
					continue
				}

				// 监听 Write/Create/Rename 事件（vim/nano 等编辑器使用 rename 保存）
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
					// 防抖：延迟执行，避免编辑器多次写入
					if debounceTimer != nil {
						debounceTimer.Stop()
					}

					debounceTimer = time.AfterFunc(w.debounceTime, func() {
						logger.Info("config", "检测到配置文件变更，正在重载")
						w.reload()
					})
				}

				// 处理 Remove/Rename：重新监听，避免 inode 变化后事件丢失
				if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 && (isConfigFile || isTemplateFile) {
					if err := w.rewatchPath(eventPath); err != nil {
						logger.Error("config", "重新监听目录失败", "error", err)
					}
				}

			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}
				// 不使用 log.Fatal，只记录错误
				logger.Error("config", "监听错误", "error", err)
			}
		}
	}()

	return nil
}

// reload 重新加载配置
func (w *Watcher) reload() {
	newConfig, err := w.loader.loadOrRollback(w.filename)
	if err != nil {
		logger.Error("config", "重载失败", "error", err)
		return
	}

	logger.Info("config", "热更新成功", "monitors", len(newConfig.Monitors))

	// 回调通知
	if w.onReload != nil {
		w.onReload(newConfig)
	}
}

// Stop 停止监听
func (w *Watcher) Stop() error {
	return w.watcher.Close()
}

// addWatch 为目录添加监听（自动去重）
func (w *Watcher) addWatch(dir string) error {
	dir = filepath.Clean(dir)
	if dir == "" {
		return nil
	}

	w.watchMu.Lock()
	if w.watchedDirs == nil {
		w.watchedDirs = make(map[string]struct{})
	}
	if _, exists := w.watchedDirs[dir]; exists {
		w.watchMu.Unlock()
		return nil
	}
	w.watchMu.Unlock()

	if err := w.watcher.Add(dir); err != nil {
		return err
	}

	w.watchMu.Lock()
	w.watchedDirs[dir] = struct{}{}
	w.watchMu.Unlock()
	return nil
}

// rewatchPath 确保替换后的文件所在目录继续被监听
func (w *Watcher) rewatchPath(path string) error {
	if path == "" {
		return nil
	}
	return w.addWatch(filepath.Dir(path))
}
