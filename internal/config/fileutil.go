package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AtomicWriteYAML 原子写入 YAML 文件（temp + fsync + rename）。
// 默认使用 0600 权限，因为 monitor 文件可能包含 API Key 等敏感信息。
func AtomicWriteYAML(path string, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("序列化 YAML 失败: %w", err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".relay-pulse-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()

	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("fsync 临时文件失败: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}

	// 设置文件权限 0600（仅属主读写，因为包含 API Key）
	if err := os.Chmod(tmpPath, 0600); err != nil {
		return fmt.Errorf("设置文件权限失败: %w", err)
	}

	// 原子替换
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("原子替换文件失败: %w", err)
	}
	tmpPath = "" // rename 成功，不再删除

	// fsync 父目录，确保 rename 操作持久化（防止掉电丢失）
	if dirHandle, err := os.Open(dir); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}

	return nil
}
