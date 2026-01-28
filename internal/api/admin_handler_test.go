package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"

	"monitor/internal/config"
	"monitor/internal/storage"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupAdminTestEnv 设置测试环境
func setupAdminTestEnv(t *testing.T) (*storage.SQLiteStorage, func()) {
	t.Helper()

	// 创建临时数据库
	tmpFile, err := os.CreateTemp("", "admin_test_*.db")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	tmpFile.Close()

	store, err := storage.NewSQLiteStorage(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("创建存储失败: %v", err)
	}

	// 初始化数据库表（包含 admin config tables）
	if err := store.Init(); err != nil {
		store.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("初始化数据库表失败: %v", err)
	}

	// 设置测试用加密密钥（32 字节 Base64 编码）
	os.Setenv(config.ConfigEncryptionKeyEnv, "dGVzdC1rZXktMzItYnl0ZXMtZm9yLWFlczI1Ng==")
	os.Setenv(AdminTokenEnvVar, "test-token-12345")

	cleanup := func() {
		store.Close()
		os.Remove(tmpFile.Name())
		os.Unsetenv(config.ConfigEncryptionKeyEnv)
		os.Unsetenv(AdminTokenEnvVar)
		config.ClearEncryptionKeyCache()
	}

	return store, cleanup
}

// setupAdminRouter 创建带认证的测试路由
func setupAdminRouter(store *storage.SQLiteStorage) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	adminHandler := NewAdminHandler(store)
	adminGroup := router.Group("/api/admin")
	adminGroup.Use(AdminAuthMiddleware())

	adminGroup.GET("/monitors", adminHandler.ListMonitors)
	adminGroup.POST("/monitors", adminHandler.CreateMonitor)
	adminGroup.GET("/monitors/:id", adminHandler.GetMonitor)
	adminGroup.PUT("/monitors/:id", adminHandler.UpdateMonitor)
	adminGroup.PATCH("/monitors/:id/status", adminHandler.UpdateMonitorStatus)
	adminGroup.DELETE("/monitors/:id", adminHandler.DeleteMonitor)
	adminGroup.POST("/monitors/:id/restore", adminHandler.RestoreMonitor)
	adminGroup.POST("/monitors/batch", adminHandler.BatchMonitors)
	adminGroup.GET("/monitors/:id/history", adminHandler.GetMonitorHistory)
	adminGroup.GET("/config/version", adminHandler.GetConfigVersion)

	return router
}

// TestAdminAuthMiddleware 测试认证中间件
func TestAdminAuthMiddleware(t *testing.T) {
	store, cleanup := setupAdminTestEnv(t)
	defer cleanup()

	router := setupAdminRouter(store)

	tests := []struct {
		name           string
		token          string
		expectedStatus int
	}{
		{
			name:           "无 Token",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "错误 Token",
			token:          "wrong-token",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "正确 Token",
			token:          "test-token-12345",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/admin/monitors", nil)
			if tt.token != "" {
				req.Header.Set(AdminTokenHeader, tt.token)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("期望状态码 %d，实际 %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestAdminMonitorCRUD 测试监测项 CRUD 操作
func TestAdminMonitorCRUD(t *testing.T) {
	store, cleanup := setupAdminTestEnv(t)
	defer cleanup()

	router := setupAdminRouter(store)

	// 1. 创建监测项
	t.Run("创建监测项", func(t *testing.T) {
		createReq := CreateMonitorRequest{
			Provider: "test-provider",
			Service:  "cc",
			Channel:  "default",
			Name:     "Test Monitor",
			Config: map[string]any{
				"url":    "https://api.example.com/v1/chat",
				"method": "POST",
			},
		}
		body, _ := json.Marshal(createReq)

		req, _ := http.NewRequest("POST", "/api/admin/monitors", bytes.NewReader(body))
		req.Header.Set(AdminTokenHeader, "test-token-12345")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("创建监测项失败，状态码 %d，响应: %s", w.Code, w.Body.String())
			return
		}

		var resp struct {
			Data struct {
				ID       int64  `json:"id"`
				Provider string `json:"provider"`
				Service  string `json:"service"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Errorf("解析响应失败: %v", err)
			return
		}

		if resp.Data.ID <= 0 {
			t.Error("创建的监测项 ID 应该 > 0")
		}
		if resp.Data.Provider != "test-provider" {
			t.Errorf("Provider 不匹配: %s", resp.Data.Provider)
		}
	})

	// 2. 获取监测项列表
	t.Run("获取监测项列表", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/admin/monitors", nil)
		req.Header.Set(AdminTokenHeader, "test-token-12345")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("获取列表失败，状态码 %d", w.Code)
			return
		}

		var resp struct {
			Data  []any `json:"data"`
			Total int   `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Errorf("解析响应失败: %v", err)
			return
		}

		if resp.Total != 1 {
			t.Errorf("期望 1 个监测项，实际 %d 个", resp.Total)
		}
	})

	// 3. 获取单个监测项
	t.Run("获取单个监测项", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/admin/monitors/1", nil)
		req.Header.Set(AdminTokenHeader, "test-token-12345")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("获取监测项失败，状态码 %d", w.Code)
			return
		}

		var resp struct {
			Data struct {
				ID       int64  `json:"id"`
				Provider string `json:"provider"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Errorf("解析响应失败: %v", err)
			return
		}

		if resp.Data.ID != 1 {
			t.Errorf("ID 不匹配: %d", resp.Data.ID)
		}
	})

	// 4. 更新监测项状态
	t.Run("更新监测项状态", func(t *testing.T) {
		enabled := false
		updateReq := struct {
			Enabled *bool `json:"enabled"`
		}{
			Enabled: &enabled,
		}
		body, _ := json.Marshal(updateReq)

		req, _ := http.NewRequest("PATCH", "/api/admin/monitors/1/status", bytes.NewReader(body))
		req.Header.Set(AdminTokenHeader, "test-token-12345")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("更新状态失败，状态码 %d，响应: %s", w.Code, w.Body.String())
		}
	})

	// 5. 删除监测项
	t.Run("删除监测项", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/admin/monitors/1", nil)
		req.Header.Set(AdminTokenHeader, "test-token-12345")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("删除监测项失败，状态码 %d", w.Code)
		}
	})

	// 6. 恢复监测项
	t.Run("恢复监测项", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/admin/monitors/1/restore", nil)
		req.Header.Set(AdminTokenHeader, "test-token-12345")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("恢复监测项失败，状态码 %d，响应: %s", w.Code, w.Body.String())
		}
	})
}

// TestAdminMonitorValidation 测试监测项验证
func TestAdminMonitorValidation(t *testing.T) {
	store, cleanup := setupAdminTestEnv(t)
	defer cleanup()

	router := setupAdminRouter(store)

	tests := []struct {
		name           string
		request        CreateMonitorRequest
		expectedStatus int
	}{
		{
			name: "缺少 provider",
			request: CreateMonitorRequest{
				Service: "cc",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "缺少 service",
			request: CreateMonitorRequest{
				Provider: "test",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "有效请求",
			request: CreateMonitorRequest{
				Provider: "test",
				Service:  "cc",
			},
			expectedStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)

			req, _ := http.NewRequest("POST", "/api/admin/monitors", bytes.NewReader(body))
			req.Header.Set(AdminTokenHeader, "test-token-12345")
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("期望状态码 %d，实际 %d，响应: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestAdminMonitorDuplicate 测试重复创建监测项
func TestAdminMonitorDuplicate(t *testing.T) {
	store, cleanup := setupAdminTestEnv(t)
	defer cleanup()

	router := setupAdminRouter(store)

	createReq := CreateMonitorRequest{
		Provider: "duplicate-test",
		Service:  "cc",
	}
	body, _ := json.Marshal(createReq)

	// 第一次创建
	req1, _ := http.NewRequest("POST", "/api/admin/monitors", bytes.NewReader(body))
	req1.Header.Set(AdminTokenHeader, "test-token-12345")
	req1.Header.Set("Content-Type", "application/json")

	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("第一次创建失败，状态码 %d", w1.Code)
	}

	// 第二次创建（应返回冲突）
	req2, _ := http.NewRequest("POST", "/api/admin/monitors", bytes.NewReader(body))
	req2.Header.Set(AdminTokenHeader, "test-token-12345")
	req2.Header.Set("Content-Type", "application/json")

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("期望冲突状态码 409，实际 %d，响应: %s", w2.Code, w2.Body.String())
	}
}

// TestAdminBatchOperations 测试批量操作
func TestAdminBatchOperations(t *testing.T) {
	store, cleanup := setupAdminTestEnv(t)
	defer cleanup()

	router := setupAdminRouter(store)

	// 创建两个监测项
	for i := 1; i <= 2; i++ {
		createReq := CreateMonitorRequest{
			Provider: "batch-test",
			Service:  "cc",
			Channel:  string(rune('a' + i - 1)), // a, b
		}
		body, _ := json.Marshal(createReq)

		req, _ := http.NewRequest("POST", "/api/admin/monitors", bytes.NewReader(body))
		req.Header.Set(AdminTokenHeader, "test-token-12345")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("创建监测项 %d 失败: %s", i, w.Body.String())
		}
	}

	// 批量禁用
	t.Run("批量禁用", func(t *testing.T) {
		batchReq := BatchMonitorRequest{
			Operations: []BatchOperation{
				{Action: "disable", MonitorID: 1},
				{Action: "disable", MonitorID: 2},
			},
		}
		body, _ := json.Marshal(batchReq)

		req, _ := http.NewRequest("POST", "/api/admin/monitors/batch", bytes.NewReader(body))
		req.Header.Set(AdminTokenHeader, "test-token-12345")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("批量操作失败，状态码 %d，响应: %s", w.Code, w.Body.String())
			return
		}

		var resp struct {
			BatchID string `json:"batch_id"`
			Total   int    `json:"total"`
			Success int    `json:"success"`
			Failed  int    `json:"failed"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Errorf("解析响应失败: %v", err)
			return
		}

		if resp.Success != 2 {
			t.Errorf("期望成功 2 个，实际 %d 个", resp.Success)
		}
		if resp.BatchID == "" {
			t.Error("batch_id 不应为空")
		}
	})
}

// TestAdminGetMonitorHistory 测试获取审计历史
func TestAdminGetMonitorHistory(t *testing.T) {
	store, cleanup := setupAdminTestEnv(t)
	defer cleanup()

	router := setupAdminRouter(store)

	// 创建监测项
	createReq := CreateMonitorRequest{
		Provider: "history-test",
		Service:  "cc",
	}
	body, _ := json.Marshal(createReq)

	req, _ := http.NewRequest("POST", "/api/admin/monitors", bytes.NewReader(body))
	req.Header.Set(AdminTokenHeader, "test-token-12345")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("创建监测项失败: %s", w.Body.String())
	}

	// 获取历史
	req2, _ := http.NewRequest("GET", "/api/admin/monitors/1/history", nil)
	req2.Header.Set(AdminTokenHeader, "test-token-12345")

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("获取历史失败，状态码 %d，响应: %s", w2.Code, w2.Body.String())
		return
	}

	var resp struct {
		Data  []any `json:"data"`
		Total int   `json:"total"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Errorf("解析响应失败: %v", err)
		return
	}

	// 创建操作应产生一条审计记录
	if resp.Total < 1 {
		t.Errorf("期望至少 1 条审计记录，实际 %d 条", resp.Total)
	}
}

// TestAdminGetConfigVersion 测试获取配置版本
func TestAdminGetConfigVersion(t *testing.T) {
	store, cleanup := setupAdminTestEnv(t)
	defer cleanup()

	router := setupAdminRouter(store)

	req, _ := http.NewRequest("GET", "/api/admin/config/version", nil)
	req.Header.Set(AdminTokenHeader, "test-token-12345")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("获取配置版本失败，状态码 %d，响应: %s", w.Code, w.Body.String())
	}
}

// TestMaskAPIKey 测试 API Key 脱敏
func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "***"},
		{"1234", "***"},
		{"12345678", "***"},
		{"123456789", "1234***6789"},
		{"sk-ant-api03-xxxxxxxxxxxxxx-xxxxxxxxxxxxx", "sk-a***xxxx"},
	}

	for _, tt := range tests {
		result := maskAPIKey(tt.input)
		if result != tt.expected {
			t.Errorf("maskAPIKey(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestMaskToken 测试 Token 脱敏
func TestMaskToken(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "***"},
		{"1234", "***"},
		{"12345678", "***"},
		{"123456789", "1234***6789"},
		{"test-token-12345", "test***2345"},
	}

	for _, tt := range tests {
		result := maskToken(tt.input)
		if result != tt.expected {
			t.Errorf("maskToken(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// setupAdminRouterFull 创建完整路由（包含全局配置 API）
func setupAdminRouterFull(store *storage.SQLiteStorage) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	adminHandler := NewAdminHandler(store)
	adminGroup := router.Group("/api/admin")
	adminGroup.Use(AdminAuthMiddleware())

	// 监测项管理 API
	adminGroup.GET("/monitors", adminHandler.ListMonitors)
	adminGroup.POST("/monitors", adminHandler.CreateMonitor)

	// Provider 策略管理 API
	adminGroup.GET("/policies", adminHandler.ListProviderPolicies)
	adminGroup.POST("/policies", adminHandler.CreateProviderPolicy)
	adminGroup.DELETE("/policies/:id", adminHandler.DeleteProviderPolicy)

	// Badge 管理 API
	adminGroup.GET("/badges/definitions", adminHandler.ListBadgeDefinitions)
	adminGroup.POST("/badges/definitions", adminHandler.CreateBadgeDefinition)
	adminGroup.DELETE("/badges/definitions/:id", adminHandler.DeleteBadgeDefinition)
	adminGroup.GET("/badges/bindings", adminHandler.ListBadgeBindings)
	adminGroup.POST("/badges/bindings", adminHandler.CreateBadgeBinding)
	adminGroup.DELETE("/badges/bindings/:id", adminHandler.DeleteBadgeBinding)

	// Board 管理 API
	adminGroup.GET("/boards", adminHandler.ListBoardConfigs)

	// 全局设置 API
	adminGroup.GET("/settings/:key", adminHandler.GetGlobalSetting)
	adminGroup.PUT("/settings/:key", adminHandler.SetGlobalSetting)

	return router
}

// TestAdminProviderPolicies 测试 Provider 策略管理
func TestAdminProviderPolicies(t *testing.T) {
	store, cleanup := setupAdminTestEnv(t)
	defer cleanup()

	router := setupAdminRouterFull(store)

	// 1. 列出策略（初始为空）
	t.Run("列出策略（空）", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/admin/policies", nil)
		req.Header.Set(AdminTokenHeader, "test-token-12345")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
		}
	})

	// 2. 创建策略
	t.Run("创建策略", func(t *testing.T) {
		body := `{"policy_type":"disabled","provider":"test-provider","reason":"测试原因"}`

		req, _ := http.NewRequest("POST", "/api/admin/policies", bytes.NewReader([]byte(body)))
		req.Header.Set(AdminTokenHeader, "test-token-12345")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("创建策略失败，状态码 %d，响应: %s", w.Code, w.Body.String())
		}
	})

	// 3. 重复创建（应返回冲突）
	t.Run("重复创建策略", func(t *testing.T) {
		body := `{"policy_type":"disabled","provider":"test-provider"}`

		req, _ := http.NewRequest("POST", "/api/admin/policies", bytes.NewReader([]byte(body)))
		req.Header.Set(AdminTokenHeader, "test-token-12345")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("期望冲突状态码 409，实际 %d", w.Code)
		}
	})

	// 4. 删除策略
	t.Run("删除策略", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/admin/policies/1", nil)
		req.Header.Set(AdminTokenHeader, "test-token-12345")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("删除策略失败，状态码 %d，响应: %s", w.Code, w.Body.String())
		}
	})

	// 5. 删除不存在的策略
	t.Run("删除不存在的策略", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/admin/policies/999", nil)
		req.Header.Set(AdminTokenHeader, "test-token-12345")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("期望 404，实际 %d", w.Code)
		}
	})
}

// TestAdminBadgeDefinitions 测试徽标定义管理
func TestAdminBadgeDefinitions(t *testing.T) {
	store, cleanup := setupAdminTestEnv(t)
	defer cleanup()

	router := setupAdminRouterFull(store)

	// 1. 列出徽标
	t.Run("列出徽标定义", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/admin/badges/definitions", nil)
		req.Header.Set(AdminTokenHeader, "test-token-12345")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
		}
	})

	// 2. 创建徽标
	t.Run("创建徽标定义", func(t *testing.T) {
		body := `{
			"id": "test-badge",
			"kind": "feature",
			"weight": 10,
			"label_i18n": {"zh-CN": "测试徽标", "en-US": "Test Badge"}
		}`

		req, _ := http.NewRequest("POST", "/api/admin/badges/definitions", bytes.NewReader([]byte(body)))
		req.Header.Set(AdminTokenHeader, "test-token-12345")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("创建徽标失败，状态码 %d，响应: %s", w.Code, w.Body.String())
		}
	})

	// 3. 重复创建
	t.Run("重复创建徽标", func(t *testing.T) {
		body := `{
			"id": "test-badge",
			"kind": "feature",
			"label_i18n": {"zh-CN": "测试徽标"}
		}`

		req, _ := http.NewRequest("POST", "/api/admin/badges/definitions", bytes.NewReader([]byte(body)))
		req.Header.Set(AdminTokenHeader, "test-token-12345")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("期望冲突状态码 409，实际 %d", w.Code)
		}
	})

	// 4. 删除徽标
	t.Run("删除徽标定义", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/admin/badges/definitions/test-badge", nil)
		req.Header.Set(AdminTokenHeader, "test-token-12345")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("删除徽标失败，状态码 %d，响应: %s", w.Code, w.Body.String())
		}
	})
}

// TestAdminBadgeBindings 测试徽标绑定管理
func TestAdminBadgeBindings(t *testing.T) {
	store, cleanup := setupAdminTestEnv(t)
	defer cleanup()

	router := setupAdminRouterFull(store)

	// 先创建一个徽标定义
	body := `{"id": "bind-test", "kind": "info", "label_i18n": {"zh-CN": "绑定测试"}}`
	req, _ := http.NewRequest("POST", "/api/admin/badges/definitions", bytes.NewReader([]byte(body)))
	req.Header.Set(AdminTokenHeader, "test-token-12345")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("创建徽标定义失败: %s", w.Body.String())
	}

	// 1. 创建全局绑定
	t.Run("创建全局绑定", func(t *testing.T) {
		body := `{"badge_id": "bind-test", "scope": "global"}`

		req, _ := http.NewRequest("POST", "/api/admin/badges/bindings", bytes.NewReader([]byte(body)))
		req.Header.Set(AdminTokenHeader, "test-token-12345")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("创建绑定失败，状态码 %d，响应: %s", w.Code, w.Body.String())
		}
	})

	// 2. scope 验证错误
	t.Run("scope 验证错误", func(t *testing.T) {
		// global 时不应有 provider
		body := `{"badge_id": "bind-test", "scope": "global", "provider": "test"}`

		req, _ := http.NewRequest("POST", "/api/admin/badges/bindings", bytes.NewReader([]byte(body)))
		req.Header.Set(AdminTokenHeader, "test-token-12345")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("期望 400，实际 %d", w.Code)
		}
	})

	// 3. provider 级别绑定
	t.Run("创建 provider 级别绑定", func(t *testing.T) {
		body := `{"badge_id": "bind-test", "scope": "provider", "provider": "test-provider"}`

		req, _ := http.NewRequest("POST", "/api/admin/badges/bindings", bytes.NewReader([]byte(body)))
		req.Header.Set(AdminTokenHeader, "test-token-12345")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("创建绑定失败，状态码 %d，响应: %s", w.Code, w.Body.String())
		}
	})

	// 4. 列出绑定
	t.Run("列出绑定", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/admin/badges/bindings", nil)
		req.Header.Set(AdminTokenHeader, "test-token-12345")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
		}

		var resp struct {
			Data []any `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Errorf("解析响应失败: %v", err)
			return
		}
		if len(resp.Data) != 2 {
			t.Errorf("期望 2 个绑定，实际 %d 个", len(resp.Data))
		}
	})

	// 5. 删除绑定
	t.Run("删除绑定", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/admin/badges/bindings/1", nil)
		req.Header.Set(AdminTokenHeader, "test-token-12345")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("删除绑定失败，状态码 %d，响应: %s", w.Code, w.Body.String())
		}
	})
}

// TestAdminBoardConfigs 测试 Board 配置管理
func TestAdminBoardConfigs(t *testing.T) {
	store, cleanup := setupAdminTestEnv(t)
	defer cleanup()

	router := setupAdminRouterFull(store)

	// 列出 Board 配置
	t.Run("列出 Board 配置", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/admin/boards", nil)
		req.Header.Set(AdminTokenHeader, "test-token-12345")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 %d，实际 %d，响应: %s", http.StatusOK, w.Code, w.Body.String())
		}
	})
}

// TestAdminGlobalSettings 测试全局设置管理
func TestAdminGlobalSettings(t *testing.T) {
	store, cleanup := setupAdminTestEnv(t)
	defer cleanup()

	router := setupAdminRouterFull(store)

	// 1. 获取不存在的设置
	t.Run("获取不存在的设置", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/admin/settings/nonexistent", nil)
		req.Header.Set(AdminTokenHeader, "test-token-12345")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("期望 404，实际 %d", w.Code)
		}
	})

	// 2. 设置值
	t.Run("设置全局设置", func(t *testing.T) {
		body := `{"value": {"enabled": true, "interval": "5m"}}`

		req, _ := http.NewRequest("PUT", "/api/admin/settings/test-key", bytes.NewReader([]byte(body)))
		req.Header.Set(AdminTokenHeader, "test-token-12345")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("设置失败，状态码 %d，响应: %s", w.Code, w.Body.String())
		}
	})

	// 3. 获取设置
	t.Run("获取全局设置", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/admin/settings/test-key", nil)
		req.Header.Set(AdminTokenHeader, "test-token-12345")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
		}
	})

	// 4. 更新设置
	t.Run("更新全局设置", func(t *testing.T) {
		body := `{"value": {"enabled": false}}`

		req, _ := http.NewRequest("PUT", "/api/admin/settings/test-key", bytes.NewReader([]byte(body)))
		req.Header.Set(AdminTokenHeader, "test-token-12345")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("更新失败，状态码 %d，响应: %s", w.Code, w.Body.String())
		}
	})

	// 5. 无效 JSON 值
	t.Run("无效 JSON 值", func(t *testing.T) {
		body := `{"value": "not-a-json-object"}`

		req, _ := http.NewRequest("PUT", "/api/admin/settings/test-key", bytes.NewReader([]byte(body)))
		req.Header.Set(AdminTokenHeader, "test-token-12345")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// "not-a-json-object" 是有效的 JSON 字符串，不会报错
		// 需要验证的是 json.Valid 检查
		if w.Code != http.StatusOK {
			t.Errorf("期望 200（字符串也是有效 JSON），实际 %d", w.Code)
		}
	})
}
