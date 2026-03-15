package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MonitorStore 提供 monitors.d/ 文件级 CRUD 操作。
// 所有写操作通过 mutex 串行化，使用 AtomicWriteYAML 确保崩溃安全。
// 写入后由 fsnotify 自动触发热更新，无需手动调用 reload。
type MonitorStore struct {
	dir string     // monitors.d/ 绝对路径
	mu  sync.Mutex // 写操作串行化
}

// NewMonitorStore 创建 MonitorStore。dir 是 monitors.d/ 的绝对路径。
func NewMonitorStore(dir string) *MonitorStore {
	return &MonitorStore{dir: dir}
}

// Dir 返回 monitors.d/ 目录路径。
func (s *MonitorStore) Dir() string {
	return s.dir
}

// validateKeySegment 校验 PSC 字段不含路径分隔符或目录穿越字符。
func validateKeySegment(field, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s 不能为空", field)
	}
	if strings.ContainsAny(value, "/\\") {
		return fmt.Errorf("%s 不能包含路径分隔符", field)
	}
	if value == "." || value == ".." || strings.Contains(value, "..") {
		return fmt.Errorf("%s 不能包含 '..'", field)
	}
	return nil
}

// SanitizeMonitorKey 规范化并校验 monitor file key，防止路径穿越。
func SanitizeMonitorKey(key string) (string, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	provider, service, channel, err := ParseMonitorFileKey(key)
	if err != nil {
		return "", err
	}
	if err := validateKeySegment("provider", provider); err != nil {
		return "", err
	}
	if err := validateKeySegment("service", service); err != nil {
		return "", err
	}
	if err := validateKeySegment("channel", channel); err != nil {
		return "", err
	}
	return MonitorFileKeyFromPSC(provider, service, channel), nil
}

// findExistingPath 查找 key 对应的 .yaml 或 .yml 文件。
func (s *MonitorStore) findExistingPath(key string) (string, error) {
	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(s.dir, key+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	return "", nil
}

// MonitorSummary 是列表 API 返回的精简摘要。
type MonitorSummary struct {
	Key         string `json:"key"`
	Provider    string `json:"provider"`
	Service     string `json:"service"`
	Channel     string `json:"channel"`
	ChannelName string `json:"channel_name,omitempty"`
	ModelCount  int    `json:"model_count"`
	Disabled    bool   `json:"disabled"`
	Hidden      bool   `json:"hidden"`
	Board       string `json:"board"`
	Category    string `json:"category"`
	Template    string `json:"template"`
	Source      string `json:"source"`
	Revision    int64  `json:"revision"`
	UpdatedAt   string `json:"updated_at"`
}

// List 列出 monitors.d/ 下所有监测文件的摘要。
func (s *MonitorStore) List() ([]MonitorSummary, error) {
	_, files, err := loadMonitorsDir(filepath.Dir(s.dir))
	if err != nil {
		return nil, err
	}

	summaries := make([]MonitorSummary, 0, len(files))
	for _, f := range files {
		if len(f.Monitors) == 0 {
			continue
		}

		root := f.Monitors[0]
		// 找到父通道（无 parent 字段的那个）
		for _, m := range f.Monitors {
			if strings.TrimSpace(m.Parent) == "" {
				root = m
				break
			}
		}

		summaries = append(summaries, MonitorSummary{
			Key:         f.Key,
			Provider:    root.Provider,
			Service:     root.Service,
			Channel:     root.Channel,
			ChannelName: root.ChannelName,
			ModelCount:  len(f.Monitors),
			Disabled:    root.Disabled,
			Hidden:      root.Hidden,
			Board:       root.Board,
			Category:    root.Category,
			Template:    root.Template,
			Source:      f.Metadata.Source,
			Revision:    f.Metadata.Revision,
			UpdatedAt:   f.Metadata.UpdatedAt,
		})
	}
	return summaries, nil
}

// Get 读取指定 key 的监测文件。key 格式: provider--service--channel
func (s *MonitorStore) Get(key string) (*MonitorFile, error) {
	var err error
	key, err = SanitizeMonitorKey(key)
	if err != nil {
		return nil, err
	}

	path, err := s.findExistingPath(key)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}

	file, err := loadMonitorFile(path)
	if err != nil {
		return nil, err
	}
	file.Path = path
	file.Key = key
	return &file, nil
}

// Create 创建新监测文件。PSC 不能已存在于 monitors.d/ 中。
func (s *MonitorStore) Create(file *MonitorFile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, err := DeriveMonitorFileKey(*file)
	if err != nil {
		return fmt.Errorf("推导 PSC key 失败: %w", err)
	}
	key, err = SanitizeMonitorKey(key)
	if err != nil {
		return fmt.Errorf("PSC key 无效: %w", err)
	}

	existing, err := s.findExistingPath(key)
	if err != nil {
		return err
	}
	if existing != "" {
		return fmt.Errorf("PSC %s 已存在", key)
	}

	path := filepath.Join(s.dir, key+".yaml")

	now := time.Now().UTC().Format(time.RFC3339)
	file.Metadata.Revision = 1
	if file.Metadata.CreatedAt == "" {
		file.Metadata.CreatedAt = now
	}
	file.Metadata.UpdatedAt = now

	if err := AtomicWriteYAML(path, file); err != nil {
		return err
	}

	file.Path = path
	file.Key = key
	return nil
}

// Update 更新监测文件。使用 revision 乐观锁防止并发覆盖。
func (s *MonitorStore) Update(key string, file *MonitorFile, expectedRevision int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	key, err = SanitizeMonitorKey(key)
	if err != nil {
		return err
	}

	path, err := s.findExistingPath(key)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("PSC %s 不存在", key)
	}

	existing, err := loadMonitorFile(path)
	if err != nil {
		return err
	}

	if existing.Metadata.Revision != expectedRevision {
		return fmt.Errorf("revision 不匹配: 期望 %d，实际 %d（文件已被其他操作修改）",
			expectedRevision, existing.Metadata.Revision)
	}

	// 校验 PSC 不可变：更新后的内容推导出的 key 必须与 URL key 一致
	newKey, err := DeriveMonitorFileKey(*file)
	if err != nil {
		return fmt.Errorf("推导 PSC key 失败: %w", err)
	}
	newKey, err = SanitizeMonitorKey(newKey)
	if err != nil {
		return fmt.Errorf("PSC key 无效: %w", err)
	}
	if newKey != key {
		return fmt.Errorf("PSC 不可变更: %s -> %s", key, newKey)
	}

	file.Metadata.Revision = expectedRevision + 1
	file.Metadata.CreatedAt = existing.Metadata.CreatedAt // 保留创建时间
	file.Metadata.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := AtomicWriteYAML(path, file); err != nil {
		return err
	}

	file.Path = path
	file.Key = key
	return nil
}

// Delete 归档删除：移动到 monitors.d/.archive/{filename}.{timestamp}.yaml。
func (s *MonitorStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	key, err = SanitizeMonitorKey(key)
	if err != nil {
		return err
	}

	path, err := s.findExistingPath(key)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("PSC %s 不存在", key)
	}

	archiveDir := filepath.Join(s.dir, ".archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return fmt.Errorf("创建 .archive 目录失败: %w", err)
	}

	ts := time.Now().UTC().Format("20060102T150405Z")
	archiveName := fmt.Sprintf("%s.%s%s", key, ts, filepath.Ext(path))
	archivePath := filepath.Join(archiveDir, archiveName)

	if err := os.Rename(path, archivePath); err != nil {
		return fmt.Errorf("归档文件失败: %w", err)
	}

	return nil
}
