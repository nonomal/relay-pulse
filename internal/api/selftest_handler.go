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
	TestType       string `json:"test_type" binding:"required"`
	PayloadVariant string `json:"payload_variant" binding:"omitempty,max=64"`
	APIURL         string `json:"api_url" binding:"required,url,max=500"`
	APIKey         string `json:"api_key" binding:"required,min=10,max=200"`
}

// CreateTestResponse 创建测试响应
type CreateTestResponse struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	PayloadVariant string `json:"payload_variant"`
	QueuePos       int    `json:"queue_position,omitempty"`
	CreatedAt      int64  `json:"created_at"`
}

// GetTestResponse 获取测试响应
type GetTestResponse struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	QueuePos       int    `json:"queue_position,omitempty"`
	TestType       string `json:"test_type"`
	PayloadVariant string `json:"payload_variant"`

	// 结果字段（完成后有值）
	ProbeStatus     *int    `json:"probe_status,omitempty"`
	SubStatus       *string `json:"sub_status,omitempty"`
	HTTPCode        *int    `json:"http_code,omitempty"`
	Latency         *int    `json:"latency,omitempty"`
	ErrorMessage    *string `json:"error_message,omitempty"`
	ResponseSnippet *string `json:"response_snippet,omitempty"` // 服务端响应片段

	// 测试证明（仅当 probe_status=1 且启用了 onboarding 时返回）
	TestProof *string `json:"test_proof,omitempty"`

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
	ID             string                     `json:"id"`
	Name           string                     `json:"name"`
	Description    string                     `json:"description"`
	DefaultVariant string                     `json:"default_variant"`
	Variants       []*selftest.PayloadVariant `json:"variants"`
}

// selfTestErrorStatusAndCode 将 selftest 领域错误码映射为 HTTP 状态码和统一 API 错误码
func selfTestErrorStatusAndCode(code selftest.ErrorCode) (int, string) {
	switch code {
	case selftest.ErrCodeBadRequest, selftest.ErrCodeInvalidURL, selftest.ErrCodeUnknownTestType, selftest.ErrCodeUnknownVariant:
		return http.StatusBadRequest, ErrCodeInvalidParam
	case selftest.ErrCodeRateLimited:
		return http.StatusTooManyRequests, ErrCodeRateLimited
	case selftest.ErrCodeFeatureDisabled:
		return http.StatusServiceUnavailable, ErrCodeFeatureDisabled
	case selftest.ErrCodeQueueFull:
		return http.StatusServiceUnavailable, ErrCodeQueueFull
	case selftest.ErrCodeJobNotFound:
		return http.StatusNotFound, ErrCodeNotFound
	default:
		return http.StatusInternalServerError, ErrCodeInternalError
	}
}

// writeSelfTestError 将 selftest 错误转换为统一 API 错误响应
func writeSelfTestError(c *gin.Context, err error, fallbackMessage string) {
	var stErr *selftest.Error
	if errors.As(err, &stErr) {
		statusCode, code := selfTestErrorStatusAndCode(stErr.Code)
		message := stErr.Message
		if code == ErrCodeInternalError || message == "" {
			message = fallbackMessage
		}
		apiError(c, statusCode, code, message)
		return
	}
	apiError(c, http.StatusInternalServerError, ErrCodeInternalError, fallbackMessage)
}

// CreateSelfTest 创建自助测试任务
// POST /api/selftest
func (h *Handler) CreateSelfTest(c *gin.Context) {
	// 检查功能是否启用（通过 mutex 安全读取，支持热更新替换实例）
	mgr := h.getSelfTestManager()
	if mgr == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "自助测试功能未启用")
		return
	}

	// IP 限流检查
	clientIP := c.ClientIP()
	if !mgr.CheckRateLimit(clientIP) {
		logger.Warn("selftest", "Rate limit exceeded", "ip", clientIP)
		apiError(c, http.StatusTooManyRequests, ErrCodeRateLimited, "请求过于频繁，请稍后再试")
		return
	}

	// 解析请求
	var req CreateTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "请求参数无效，请检查 test_type、api_url、api_key 字段")
		return
	}

	// 创建任务
	job, err := mgr.CreateJob(
		req.TestType,
		req.APIURL,
		req.APIKey,
		req.PayloadVariant,
	)
	if err != nil {
		logger.Error("selftest", "Failed to create job",
			"test_type", req.TestType, "ip", clientIP, "error", err)
		writeSelfTestError(c, err, "创建自助测试任务失败，请稍后再试")
		return
	}

	// 返回响应
	resp := CreateTestResponse{
		ID:             job.ID,
		Status:         string(job.Status),
		PayloadVariant: job.PayloadVariant,
		QueuePos:       job.QueuePos,
		CreatedAt:      job.CreatedAt.Unix(),
	}

	logger.Info("selftest", "Job created",
		"job_id", job.ID, "test_type", req.TestType, "ip", clientIP)

	c.JSON(http.StatusCreated, resp)
}

// GetSelfTest 获取测试任务状态
// GET /api/selftest/:id
func (h *Handler) GetSelfTest(c *gin.Context) {
	mgr := h.getSelfTestManager()
	if mgr == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "自助测试功能未启用")
		return
	}

	jobID := c.Param("id")
	if jobID == "" {
		apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "任务 ID 不能为空")
		return
	}

	// 获取任务
	job, err := mgr.GetJob(jobID)
	if err != nil {
		writeSelfTestError(c, err, "查询自助测试任务失败，请稍后再试")
		return
	}

	// 构造响应
	resp := GetTestResponse{
		ID:             job.ID,
		Status:         string(job.Status),
		QueuePos:       job.QueuePos,
		TestType:       job.TestType,
		PayloadVariant: job.PayloadVariant,
		CreatedAt:      job.CreatedAt.Unix(),
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

		// 测试通过时签发 proof（供 onboarding/change 提交使用）
		if job.ProbeStatus == 1 {
			apiKey := mgr.GetJobAPIKey(jobID)
			if apiKey != "" {
				// 优先使用 onboarding 服务签发，fallback 到 change 服务
				if svc := h.getOnboardingService(); svc != nil {
					proof := svc.IssueProof(jobID, job.TestType, job.APIURL, apiKey)
					resp.TestProof = &proof
				} else if chSvc := h.getChangeService(); chSvc != nil {
					proof := chSvc.IssueProof(jobID, job.TestType, job.APIURL, apiKey)
					resp.TestProof = &proof
				}
			}
		}
	}

	c.JSON(http.StatusOK, resp)
}

// GetSelfTestConfig 获取自助测试配置
// GET /api/selftest/config
func (h *Handler) GetSelfTestConfig(c *gin.Context) {
	if h.getSelfTestManager() == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "自助测试功能未启用")
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
	if h.getSelfTestManager() == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "自助测试功能未启用")
		return
	}

	types := selftest.ListTestTypes()
	var response []TestTypeInfo
	for _, t := range types {
		response = append(response, TestTypeInfo{
			ID:             t.ID,
			Name:           t.Name,
			Description:    t.Description,
			DefaultVariant: t.DefaultVariant,
			Variants:       t.Variants,
		})
	}

	c.JSON(http.StatusOK, response)
}
