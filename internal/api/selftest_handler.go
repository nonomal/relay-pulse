package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"monitor/internal/logger"
	"monitor/internal/selftest"
)

// CreateTestRequest 创建测试请求
type CreateTestRequest struct {
	TestType string `json:"test_type" binding:"required"`
	APIURL   string `json:"api_url" binding:"required,url,max=500"`
	APIKey   string `json:"api_key" binding:"required,min=10,max=200"`
}

// ErrorResponse 自助测试错误响应（兼容旧版，仅在需要时附带 code）
type ErrorResponse struct {
	Code  string `json:"code,omitempty"`
	Error string `json:"error"`
}

// CreateTestResponse 创建测试响应
type CreateTestResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	QueuePos  int    `json:"queue_position,omitempty"`
	CreatedAt int64  `json:"created_at"`
}

// GetTestResponse 获取测试响应
type GetTestResponse struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	QueuePos int    `json:"queue_position,omitempty"`
	TestType string `json:"test_type"`

	// 结果字段（完成后有值）
	ProbeStatus     *int    `json:"probe_status,omitempty"`
	SubStatus       *string `json:"sub_status,omitempty"`
	HTTPCode        *int    `json:"http_code,omitempty"`
	Latency         *int    `json:"latency,omitempty"`
	ErrorMessage    *string `json:"error_message,omitempty"`
	ResponseSnippet *string `json:"response_snippet,omitempty"` // 服务端响应片段

	CreatedAt  int64  `json:"created_at"`
	StartedAt  *int64 `json:"started_at,omitempty"`
	FinishedAt *int64 `json:"finished_at,omitempty"`
}

// SelfTestConfigResponse 自助测试配置响应
type SelfTestConfigResponse struct {
	MaxConcurrent      int `json:"max_concurrent"`
	MaxQueueSize       int `json:"max_queue_size"`
	JobTimeoutSeconds  int `json:"job_timeout_seconds"`
	RateLimitPerMinute int `json:"rate_limit_per_minute"`
	// 签名密钥不应暴露给客户端
}

// TestTypeInfo 测试类型信息
type TestTypeInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CreateSelfTest 创建自助测试任务
// POST /api/selftest
func (h *Handler) CreateSelfTest(c *gin.Context) {
	// 检查功能是否启用
	if h.selfTestMgr == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Code:  string(selftest.ErrCodeFeatureDisabled),
			Error: "自助测试功能未启用",
		})
		return
	}

	// IP 限流检查
	clientIP := c.ClientIP()
	if !h.selfTestMgr.CheckRateLimit(clientIP) {
		logger.Warn("selftest", "Rate limit exceeded", "ip", clientIP)
		c.JSON(http.StatusTooManyRequests, ErrorResponse{
			Code:  string(selftest.ErrCodeRateLimited),
			Error: "请求过于频繁，请稍后再试",
		})
		return
	}

	// 解析请求
	var req CreateTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  string(selftest.ErrCodeBadRequest),
			Error: err.Error(),
		})
		return
	}

	// 创建任务
	job, err := h.selfTestMgr.CreateJob(
		req.TestType,
		req.APIURL,
		req.APIKey,
	)
	if err != nil {
		logger.Error("selftest", "Failed to create job",
			"test_type", req.TestType, "ip", clientIP, "error", err)

		// 根据错误码返回不同的状态码
		statusCode := http.StatusBadRequest
		code := selftest.CodeOf(err)
		switch code {
		case selftest.ErrCodeQueueFull:
			statusCode = http.StatusServiceUnavailable
		case selftest.ErrCodeInvalidURL, selftest.ErrCodeUnknownTestType:
			statusCode = http.StatusBadRequest
		}

		// 尝试提取结构化错误
		var stErr *selftest.Error
		if errors.As(err, &stErr) {
			c.JSON(statusCode, ErrorResponse{
				Code:  string(stErr.Code),
				Error: stErr.Message,
			})
			return
		}

		c.JSON(statusCode, ErrorResponse{
			Code:  string(code),
			Error: err.Error(),
		})
		return
	}

	// 返回响应
	resp := CreateTestResponse{
		ID:        job.ID,
		Status:    string(job.Status),
		QueuePos:  job.QueuePos,
		CreatedAt: job.CreatedAt.Unix(),
	}

	logger.Info("selftest", "Job created",
		"job_id", job.ID, "test_type", req.TestType, "ip", clientIP)

	c.JSON(http.StatusCreated, resp)
}

// GetSelfTest 获取测试任务状态
// GET /api/selftest/:id
func (h *Handler) GetSelfTest(c *gin.Context) {
	if h.selfTestMgr == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Code:  string(selftest.ErrCodeFeatureDisabled),
			Error: "自助测试功能未启用",
		})
		return
	}

	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  string(selftest.ErrCodeBadRequest),
			Error: "job_id is required",
		})
		return
	}

	// 获取任务
	job, err := h.selfTestMgr.GetJob(jobID)
	if err != nil {
		code := selftest.CodeOf(err)
		var stErr *selftest.Error
		if errors.As(err, &stErr) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Code:  string(stErr.Code),
				Error: stErr.Message,
			})
			return
		}
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:  string(code),
			Error: err.Error(),
		})
		return
	}

	// 构造响应
	resp := GetTestResponse{
		ID:        job.ID,
		Status:    string(job.Status),
		QueuePos:  job.QueuePos,
		TestType:  job.TestType,
		CreatedAt: job.CreatedAt.Unix(),
	}

	// 如果已开始，添加开始时间
	if job.StartedAt != nil {
		startedAt := job.StartedAt.Unix()
		resp.StartedAt = &startedAt
	}

	// 如果已完成，添加结果和完成时间
	if job.IsTerminal() {
		resp.ProbeStatus = &job.ProbeStatus
		if job.SubStatus != "" {
			resp.SubStatus = &job.SubStatus
		}
		if job.HTTPCode > 0 {
			resp.HTTPCode = &job.HTTPCode
		}
		if job.Latency > 0 {
			resp.Latency = &job.Latency
		}
		if job.ErrorMessage != "" {
			resp.ErrorMessage = &job.ErrorMessage
		}
		if job.ResponseSnippet != "" {
			resp.ResponseSnippet = &job.ResponseSnippet
		}
		if job.FinishedAt != nil {
			finishedAt := job.FinishedAt.Unix()
			resp.FinishedAt = &finishedAt
		}
	}

	c.JSON(http.StatusOK, resp)
}

// GetSelfTestConfig 获取自助测试配置
// GET /api/selftest/config
func (h *Handler) GetSelfTestConfig(c *gin.Context) {
	if h.selfTestMgr == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Code:  string(selftest.ErrCodeFeatureDisabled),
			Error: "自助测试功能未启用",
		})
		return
	}

	h.cfgMu.RLock()
	cfg := h.config.SelfTest
	h.cfgMu.RUnlock()

	resp := SelfTestConfigResponse{
		MaxConcurrent:      cfg.MaxConcurrent,
		MaxQueueSize:       cfg.MaxQueueSize,
		JobTimeoutSeconds:  int(cfg.JobTimeoutDuration.Seconds()),
		RateLimitPerMinute: cfg.RateLimitPerMinute,
		// SignatureSecret 不应暴露给客户端
	}

	c.JSON(http.StatusOK, resp)
}

// GetTestTypes 获取可用的测试类型
// GET /api/selftest/types
func (h *Handler) GetTestTypes(c *gin.Context) {
	if h.selfTestMgr == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{
			Code:  string(selftest.ErrCodeFeatureDisabled),
			Error: "自助测试功能未启用",
		})
		return
	}

	types := selftest.ListTestTypes()
	var response []TestTypeInfo
	for _, t := range types {
		response = append(response, TestTypeInfo{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
		})
	}

	c.JSON(http.StatusOK, response)
}
