package storage

import (
	"os"
	"testing"
)

func TestAdminConfigTables(t *testing.T) {
	// 使用临时数据库文件
	tmpFile, err := os.CreateTemp("", "test-admin-config-*.db")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// 创建 SQLite 存储
	storage, err := NewSQLiteStorage(tmpPath)
	if err != nil {
		t.Fatalf("创建 SQLite 存储失败: %v", err)
	}
	defer storage.Close()

	// 初始化表
	if err := storage.Init(); err != nil {
		t.Fatalf("初始化表失败: %v", err)
	}

	// 测试获取配置版本
	t.Run("GetConfigVersions", func(t *testing.T) {
		versions, err := storage.GetConfigVersions()
		if err != nil {
			t.Fatalf("获取配置版本失败: %v", err)
		}

		if versions.Monitors != 1 {
			t.Errorf("monitors 版本应为 1，实际为 %d", versions.Monitors)
		}
		if versions.Policies != 1 {
			t.Errorf("policies 版本应为 1，实际为 %d", versions.Policies)
		}
		if versions.Badges != 1 {
			t.Errorf("badges 版本应为 1，实际为 %d", versions.Badges)
		}
		if versions.Boards != 1 {
			t.Errorf("boards 版本应为 1，实际为 %d", versions.Boards)
		}
		if versions.Settings != 1 {
			t.Errorf("settings 版本应为 1，实际为 %d", versions.Settings)
		}
	})

	// 测试递增配置版本
	t.Run("IncrementConfigVersion", func(t *testing.T) {
		if err := storage.IncrementConfigVersion(ConfigScopeMonitors); err != nil {
			t.Fatalf("递增配置版本失败: %v", err)
		}

		versions, err := storage.GetConfigVersions()
		if err != nil {
			t.Fatalf("获取配置版本失败: %v", err)
		}

		if versions.Monitors != 2 {
			t.Errorf("monitors 版本应为 2，实际为 %d", versions.Monitors)
		}
	})
}

func TestMonitorConfigCRUD(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-monitor-config-*.db")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	storage, err := NewSQLiteStorage(tmpPath)
	if err != nil {
		t.Fatalf("创建 SQLite 存储失败: %v", err)
	}
	defer storage.Close()

	if err := storage.Init(); err != nil {
		t.Fatalf("初始化表失败: %v", err)
	}

	// 测试创建
	t.Run("Create", func(t *testing.T) {
		config := &MonitorConfig{
			Provider:   "test-provider",
			Service:    "test-service",
			Channel:    "test-channel",
			Model:      "",
			Name:       "Test Monitor",
			Enabled:    true,
			ConfigBlob: `{"url": "https://example.com"}`,
		}

		if err := storage.CreateMonitorConfig(config); err != nil {
			t.Fatalf("创建监测项配置失败: %v", err)
		}

		if config.ID <= 0 {
			t.Error("创建后 ID 应 > 0")
		}
		if config.Version != 1 {
			t.Errorf("版本应为 1，实际为 %d", config.Version)
		}
	})

	// 测试按 ID 获取
	t.Run("GetByID", func(t *testing.T) {
		config, err := storage.GetMonitorConfig(1)
		if err != nil {
			t.Fatalf("获取监测项配置失败: %v", err)
		}
		if config == nil {
			t.Fatal("配置不应为空")
		}
		if config.Provider != "test-provider" {
			t.Errorf("provider 不匹配: %s", config.Provider)
		}
		if config.Name != "Test Monitor" {
			t.Errorf("name 不匹配: %s", config.Name)
		}
		if !config.Enabled {
			t.Error("enabled 应为 true")
		}
	})

	// 测试按四元组获取
	t.Run("GetByKey", func(t *testing.T) {
		config, err := storage.GetMonitorConfigByKey("test-provider", "test-service", "test-channel", "")
		if err != nil {
			t.Fatalf("获取监测项配置失败: %v", err)
		}
		if config == nil {
			t.Fatal("配置不应为空")
		}
		if config.ID != 1 {
			t.Errorf("ID 应为 1，实际为 %d", config.ID)
		}
	})

	// 测试更新（乐观锁）
	t.Run("Update", func(t *testing.T) {
		config, _ := storage.GetMonitorConfig(1)
		config.Name = "Updated Name"
		config.ConfigBlob = `{"url": "https://updated.com"}`

		if err := storage.UpdateMonitorConfig(config); err != nil {
			t.Fatalf("更新监测项配置失败: %v", err)
		}

		if config.Version != 2 {
			t.Errorf("版本应为 2，实际为 %d", config.Version)
		}

		// 验证更新
		updated, _ := storage.GetMonitorConfig(1)
		if updated.Name != "Updated Name" {
			t.Errorf("name 应为 'Updated Name'，实际为 %s", updated.Name)
		}
	})

	// 测试乐观锁冲突
	t.Run("OptimisticLockConflict", func(t *testing.T) {
		config := &MonitorConfig{
			ID:         1,
			Version:    1, // 使用旧版本
			Name:       "Conflict Name",
			ConfigBlob: `{}`,
		}

		err := storage.UpdateMonitorConfig(config)
		if err == nil {
			t.Error("应返回版本冲突错误")
		}
	})

	// 测试唯一约束
	t.Run("UniqueConstraint", func(t *testing.T) {
		config := &MonitorConfig{
			Provider:   "test-provider",
			Service:    "test-service",
			Channel:    "test-channel",
			Model:      "",
			ConfigBlob: `{}`,
		}

		err := storage.CreateMonitorConfig(config)
		if err == nil {
			t.Error("应返回唯一约束冲突错误")
		}
	})

	// 测试软删除
	t.Run("Delete", func(t *testing.T) {
		if err := storage.DeleteMonitorConfig(1); err != nil {
			t.Fatalf("删除监测项配置失败: %v", err)
		}

		// 按四元组应查不到
		config, _ := storage.GetMonitorConfigByKey("test-provider", "test-service", "test-channel", "")
		if config != nil {
			t.Error("删除后按四元组不应查到")
		}

		// 按 ID 应能查到（含 deleted_at）
		config, _ = storage.GetMonitorConfig(1)
		if config == nil {
			t.Error("按 ID 应能查到已删除记录")
		}
		if config.DeletedAt == nil {
			t.Error("deleted_at 不应为空")
		}
	})

	// 测试恢复
	t.Run("Restore", func(t *testing.T) {
		if err := storage.RestoreMonitorConfig(1); err != nil {
			t.Fatalf("恢复监测项配置失败: %v", err)
		}

		config, _ := storage.GetMonitorConfigByKey("test-provider", "test-service", "test-channel", "")
		if config == nil {
			t.Error("恢复后应能查到")
		}
		if config.DeletedAt != nil {
			t.Error("恢复后 deleted_at 应为空")
		}
	})

	// 测试列表查询
	t.Run("List", func(t *testing.T) {
		// 创建更多测试数据
		for i := 1; i <= 3; i++ {
			config := &MonitorConfig{
				Provider:   "test-provider",
				Service:    "test-service",
				Channel:    "channel-" + string(rune('a'+i)),
				ConfigBlob: `{}`,
			}
			storage.CreateMonitorConfig(config)
		}

		// 查询全部
		configs, total, err := storage.ListMonitorConfigs(&MonitorConfigFilter{})
		if err != nil {
			t.Fatalf("列表查询失败: %v", err)
		}
		if total != 4 {
			t.Errorf("总数应为 4，实际为 %d", total)
		}
		if len(configs) != 4 {
			t.Errorf("结果数应为 4，实际为 %d", len(configs))
		}

		// 按 provider 过滤
		configs, total, err = storage.ListMonitorConfigs(&MonitorConfigFilter{
			Provider: "test-provider",
		})
		if err != nil {
			t.Fatalf("过滤查询失败: %v", err)
		}
		if total != 4 {
			t.Errorf("过滤后总数应为 4，实际为 %d", total)
		}

		// 分页
		configs, total, err = storage.ListMonitorConfigs(&MonitorConfigFilter{
			Limit:  2,
			Offset: 0,
		})
		if err != nil {
			t.Fatalf("分页查询失败: %v", err)
		}
		if len(configs) != 2 {
			t.Errorf("分页结果数应为 2，实际为 %d", len(configs))
		}
		if total != 4 {
			t.Errorf("分页后总数应为 4，实际为 %d", total)
		}
	})
}

func TestMonitorSecretCRUD(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-monitor-secret-*.db")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	storage, err := NewSQLiteStorage(tmpPath)
	if err != nil {
		t.Fatalf("创建 SQLite 存储失败: %v", err)
	}
	defer storage.Close()

	if err := storage.Init(); err != nil {
		t.Fatalf("初始化表失败: %v", err)
	}

	monitorID := int64(1)
	ciphertext := []byte("encrypted-api-key")
	nonce := []byte("random-nonce")
	keyVersion := 1
	encVersion := 1

	// 测试设置
	t.Run("Set", func(t *testing.T) {
		if err := storage.SetMonitorSecret(monitorID, ciphertext, nonce, keyVersion, encVersion); err != nil {
			t.Fatalf("设置密钥失败: %v", err)
		}
	})

	// 测试获取
	t.Run("Get", func(t *testing.T) {
		secret, err := storage.GetMonitorSecret(monitorID)
		if err != nil {
			t.Fatalf("获取密钥失败: %v", err)
		}
		if secret == nil {
			t.Fatal("密钥不应为空")
		}
		if string(secret.APIKeyCiphertext) != string(ciphertext) {
			t.Error("密文不匹配")
		}
		if string(secret.APIKeyNonce) != string(nonce) {
			t.Error("nonce 不匹配")
		}
		if secret.KeyVersion != keyVersion {
			t.Errorf("key_version 应为 %d，实际为 %d", keyVersion, secret.KeyVersion)
		}
	})

	// 测试更新（upsert）
	t.Run("Update", func(t *testing.T) {
		newCiphertext := []byte("new-encrypted-key")
		newNonce := []byte("new-nonce")
		newKeyVersion := 2

		if err := storage.SetMonitorSecret(monitorID, newCiphertext, newNonce, newKeyVersion, encVersion); err != nil {
			t.Fatalf("更新密钥失败: %v", err)
		}

		secret, _ := storage.GetMonitorSecret(monitorID)
		if secret.KeyVersion != newKeyVersion {
			t.Errorf("key_version 应为 %d，实际为 %d", newKeyVersion, secret.KeyVersion)
		}
	})

	// 测试删除
	t.Run("Delete", func(t *testing.T) {
		if err := storage.DeleteMonitorSecret(monitorID); err != nil {
			t.Fatalf("删除密钥失败: %v", err)
		}

		secret, _ := storage.GetMonitorSecret(monitorID)
		if secret != nil {
			t.Error("删除后不应能获取到密钥")
		}
	})

	// 测试获取不存在的密钥
	t.Run("GetNonExistent", func(t *testing.T) {
		secret, err := storage.GetMonitorSecret(999)
		if err != nil {
			t.Fatalf("获取不存在密钥应返回 nil, nil: %v", err)
		}
		if secret != nil {
			t.Error("不存在的密钥应返回 nil")
		}
	})
}

func TestMonitorConfigAudit(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-audit-*.db")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	storage, err := NewSQLiteStorage(tmpPath)
	if err != nil {
		t.Fatalf("创建 SQLite 存储失败: %v", err)
	}
	defer storage.Close()

	if err := storage.Init(); err != nil {
		t.Fatalf("初始化表失败: %v", err)
	}

	// 创建审计记录
	t.Run("Create", func(t *testing.T) {
		audit := &MonitorConfigAudit{
			MonitorID: 1,
			Provider:  "test-provider",
			Service:   "test-service",
			Channel:   "test-channel",
			Model:     "",
			Action:    AuditActionCreate,
			AfterBlob: `{"url": "https://example.com"}`,
			Actor:     "admin",
			RequestID: "req-123",
		}

		if err := storage.CreateMonitorConfigAudit(audit); err != nil {
			t.Fatalf("创建审计记录失败: %v", err)
		}

		if audit.ID <= 0 {
			t.Error("审计记录 ID 应 > 0")
		}
	})

	// 创建更多审计记录
	for i := 0; i < 5; i++ {
		audit := &MonitorConfigAudit{
			MonitorID: 1,
			Provider:  "test-provider",
			Service:   "test-service",
			Action:    AuditActionUpdate,
			Actor:     "admin",
		}
		storage.CreateMonitorConfigAudit(audit)
	}

	// 测试列表查询
	t.Run("List", func(t *testing.T) {
		audits, total, err := storage.ListMonitorConfigAudits(&AuditFilter{})
		if err != nil {
			t.Fatalf("列表查询失败: %v", err)
		}
		if total != 6 {
			t.Errorf("总数应为 6，实际为 %d", total)
		}
		if len(audits) != 6 {
			t.Errorf("结果数应为 6，实际为 %d", len(audits))
		}
	})

	// 按 MonitorID 过滤
	t.Run("FilterByMonitorID", func(t *testing.T) {
		audits, total, err := storage.ListMonitorConfigAudits(&AuditFilter{
			MonitorID: 1,
		})
		if err != nil {
			t.Fatalf("过滤查询失败: %v", err)
		}
		if total != 6 {
			t.Errorf("总数应为 6，实际为 %d", total)
		}
		if len(audits) != 6 {
			t.Errorf("结果数应为 6，实际为 %d", len(audits))
		}
	})

	// 按 Action 过滤
	t.Run("FilterByAction", func(t *testing.T) {
		audits, total, err := storage.ListMonitorConfigAudits(&AuditFilter{
			Action: AuditActionCreate,
		})
		if err != nil {
			t.Fatalf("过滤查询失败: %v", err)
		}
		if total != 1 {
			t.Errorf("总数应为 1，实际为 %d", total)
		}
		if len(audits) != 1 {
			t.Errorf("结果数应为 1，实际为 %d", len(audits))
		}
	})
}
