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

	// monitors.d/ 目录（PSC 级 monitor 文件）
	monitorsDirPath := filepath.Clean(filepath.Join(dir, MonitorsDirName))
	monitorsDirPrefix := monitorsDirPath + string(filepath.Separator)
	if info, err := os.Stat(monitorsDirPath); err == nil && info.IsDir() {
		if err := w.addWatch(monitorsDirPath); err != nil {
			return err
		}
	}

	logger.Info("config", "开始监听配置文件",
		"file", w.filename, "dir", dir, "monitors_dir", MonitorsDirName)

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

				eventPath := filepath.Clean(event.Name)
				isConfigFile := eventPath == targetFile
				isTemplateFile := strings.HasPrefix(eventPath, templatesDirPrefix)

				// monitors.d/ 目录本身被创建/删除，或其中的 .yaml 文件变更
				isMonitorsDirSelf := eventPath == monitorsDirPath
				isMonitorDFile := strings.HasPrefix(eventPath, monitorsDirPrefix) &&
					isMonitorDefinitionFile(filepath.Base(eventPath))

				if !isConfigFile && !isTemplateFile && !isMonitorsDirSelf && !isMonitorDFile {
					continue
				}

				// 处理 monitors.d/ 目录运行时创建：自动添加监听
				if isMonitorsDirSelf && event.Op&(fsnotify.Create|fsnotify.Rename) != 0 {
					if info, err := os.Stat(monitorsDirPath); err == nil && info.IsDir() {
						if err := w.addWatch(monitorsDirPath); err != nil {
							logger.Error("config", "添加 monitors.d 目录监听失败", "error", err)
						}
					}
				}

				// 监听 Write/Create/Rename/Remove 事件触发重载
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
					debounceTimer = time.AfterFunc(w.debounceTime, func() {
						logger.Info("config", "检测到配置文件变更，正在重载")
						w.reload()
					})
				}

				// 处理 Remove/Rename：重新监听，避免 inode 变化后事件丢失
				if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
					// monitors.d/ 目录本身被删除/重命名时，清除 watchedDirs 记录
					// 使后续重新创建目录时 addWatch 能正确注册
					if isMonitorsDirSelf {
						w.removeWatch(monitorsDirPath)
					}
					if err := w.rewatchPath(eventPath); err != nil {
						logger.Error("config", "重新监听目录失败", "error", err)
					}
				}

			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}
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

// removeWatch 从 watchedDirs 缓存中移除目录记录。
// 用于目录被删除/重命名后清除过期记录，使后续 addWatch 能重新注册。
func (w *Watcher) removeWatch(dir string) {
	dir = filepath.Clean(dir)
	if dir == "" {
		return
	}
	w.watchMu.Lock()
	delete(w.watchedDirs, dir)
	w.watchMu.Unlock()
}

// rewatchPath 确保替换后的文件所在目录继续被监听
func (w *Watcher) rewatchPath(path string) error {
	if path == "" {
		return nil
	}
	return w.addWatch(filepath.Dir(path))
}
