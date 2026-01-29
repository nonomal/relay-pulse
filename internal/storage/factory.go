package storage

import (
	"fmt"
	"strings"

	"monitor/internal/config"
)

// New 创建存储实例（工厂模式）
func New(cfg *config.StorageConfig) (Storage, error) {
	storageType := strings.ToLower(strings.TrimSpace(cfg.Type))
	if storageType == "" {
		storageType = "postgres"
	}

	switch storageType {
	case "postgres", "postgresql":
		return NewPostgresStorage(&cfg.Postgres)

	default:
		return nil, fmt.Errorf("不支持的存储类型: %s (仅支持: postgres/postgresql)", cfg.Type)
	}
}
