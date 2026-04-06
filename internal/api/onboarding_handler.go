package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"monitor/internal/logger"
	"monitor/internal/onboarding"
	"monitor/internal/probe"
)

// OnboardingMetaResponse 申请表单元数据
type OnboardingMetaResponse struct {
	ServiceTypes   []string             `json:"service_types"`
	SponsorLevels  []SponsorLevelInfo   `json:"sponsor_levels"`
	ChannelTypes   []ChannelTypeInfo    `json:"channel_types"`
	ChannelSources []string             `json:"channel_sources"`
	TestTypes      []OnboardingTestType `json:"test_types"`
	ContactInfo    string               `json:"contact_info"`
}

// OnboardingTestType 收录测试类型信息
type OnboardingTestType struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Description    string              `json:"description"`
	DefaultVariant string              `json:"default_variant"`
	Variants       []OnboardingVariant `json:"variants"`
}

// OnboardingVariant 收录测试变体信息
type OnboardingVariant struct {
	ID    string `json:"id"`
	Order int    `json:"order"`
}

// SponsorLevelInfo 赞助等级信息
type SponsorLevelInfo struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// ChannelTypeInfo 通道类型信息
type ChannelTypeInfo struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// GetOnboardingMeta 获取申请表单元数据
// GET /api/onboarding/meta
func (h *Handler) GetOnboardingMeta(c *gin.Context) {
	svc := h.getOnboardingService()
	if svc == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "自助收录功能未启用")
		return
	}

	h.cfgMu.RLock()
	contactInfo := h.config.Onboarding.ContactInfo
	h.cfgMu.RUnlock()

	// 获取可用测试类型（从 probe 注册表）
	var testTypes []OnboardingTestType
	for _, t := range probe.ListTestTypes() {
		var variants []OnboardingVariant
		for _, v := range t.Variants {
			if v != nil {
				variants = append(variants, OnboardingVariant{ID: v.ID, Order: v.Order})
			}
		}
		testTypes = append(testTypes, OnboardingTestType{
			ID:             t.ID,
			Name:           t.Name,
			Description:    t.Description,
			DefaultVariant: t.DefaultVariant,
			Variants:       variants,
		})
	}

	resp := OnboardingMetaResponse{
		ServiceTypes: []string{"cc", "cx", "gm"},
		SponsorLevels: []SponsorLevelInfo{
			{Value: "public", Label: "Public", Description: "公益链路"},
			{Value: "signal", Label: "Signal", Description: "信号链路"},
			{Value: "pulse", Label: "Pulse", Description: "脉冲链路"},
		},
		ChannelTypes: []ChannelTypeInfo{
			{Value: "O", Label: "官方通道"},
			{Value: "R", Label: "逆向通道"},
			{Value: "M", Label: "混合通道"},
			{Value: "X", Label: "其他"},
		},
		ChannelSources: []string{},
		TestTypes:      testTypes,
		ContactInfo:    contactInfo,
	}

	c.JSON(http.StatusOK, resp)
}

// SubmitOnboarding 提交收录申请
// POST /api/onboarding/submit
func (h *Handler) SubmitOnboarding(c *gin.Context) {
	svc := h.getOnboardingService()
	if svc == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "自助收录功能未启用")
		return
	}

	var req onboarding.SubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn("onboarding", "提交参数校验失败", "error", err)
		apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "请求参数无效，请检查必填字段: "+err.Error())
		return
	}

	clientIP := c.ClientIP()
	resp, err := svc.Submit(c.Request.Context(), &req, clientIP)
	if err != nil {
		apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, err.Error())
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// GetOnboardingStatus 查询申请状态
// GET /api/onboarding/:id
func (h *Handler) GetOnboardingStatus(c *gin.Context) {
	svc := h.getOnboardingService()
	if svc == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "自助收录功能未启用")
		return
	}

	publicID := c.Param("id")
	if publicID == "" {
		apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "申请 ID 不能为空")
		return
	}

	sub, err := svc.GetStatus(c.Request.Context(), publicID)
	if err != nil {
		apiError(c, http.StatusInternalServerError, ErrCodeInternalError, "查询申请状态失败")
		return
	}
	if sub == nil {
		apiError(c, http.StatusNotFound, ErrCodeNotFound, "申请不存在")
		return
	}

	// 用户端只返回有限字段
	c.JSON(http.StatusOK, gin.H{
		"public_id":     sub.PublicID,
		"status":        sub.Status,
		"provider_name": sub.ProviderName,
		"service_type":  sub.ServiceType,
		"channel_code":  sub.ChannelCode,
		"created_at":    sub.CreatedAt,
		"updated_at":    sub.UpdatedAt,
	})
}

// onboardingTestRequest 内联探测请求
type onboardingTestRequest struct {
	ServiceType  string `json:"service_type" binding:"required"`
	TemplateName string `json:"template_name" binding:"required"`
	BaseURL      string `json:"base_url" binding:"required"`
	APIKey       string `json:"api_key" binding:"required"`
}

// OnboardingTest 收录内联探测测试
// POST /api/onboarding/test
func (h *Handler) OnboardingTest(c *gin.Context) {
	svc := h.getOnboardingService()
	if svc == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "自助收录功能未启用")
		return
	}

	if h.inlineProber == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "内联探测器未初始化")
		return
	}

	// IP 限流
	if h.probeLimiter != nil && !h.probeLimiter.Allow(c.ClientIP()) {
		apiError(c, http.StatusTooManyRequests, ErrCodeRateLimited, "请求过于频繁，请稍后再试")
		return
	}

	var req onboardingTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "请求参数无效: "+err.Error())
		return
	}

	// SSRF 前置校验
	guard := probe.NewSSRFGuard()
	if err := guard.ValidateURL(req.BaseURL); err != nil {
		apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "URL 安全校验失败: "+err.Error())
		return
	}

	// 30 秒总超时
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result := h.inlineProber.Probe(ctx, req.ServiceType, req.TemplateName, req.BaseURL, req.APIKey)

	resp := gin.H{
		"probe_status":     result.ProbeStatus,
		"sub_status":       result.SubStatus,
		"http_code":        result.HTTPCode,
		"latency":          result.Latency,
		"error_message":    result.ErrorMessage,
		"response_snippet": result.ResponseSnippet,
		"probe_id":         result.ProbeID,
	}

	// 探测成功时签发 proof
	if result.ProbeStatus == 1 {
		proof := svc.IssueProof(result.ProbeID, req.ServiceType, req.BaseURL, req.APIKey)
		resp["test_proof"] = proof
	}

	c.JSON(http.StatusOK, resp)
}
