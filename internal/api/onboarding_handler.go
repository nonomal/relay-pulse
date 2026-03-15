package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"monitor/internal/logger"
	"monitor/internal/onboarding"
	"monitor/internal/selftest"
)

// OnboardingMetaResponse 申请表单元数据
type OnboardingMetaResponse struct {
	ServiceTypes   []string           `json:"service_types"`
	SponsorLevels  []SponsorLevelInfo `json:"sponsor_levels"`
	ChannelTypes   []ChannelTypeInfo  `json:"channel_types"`
	ChannelSources []string           `json:"channel_sources"`
	TestTypes      []TestTypeInfo     `json:"test_types"`
	ContactInfo    string             `json:"contact_info"`
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

	// 获取 selftest 可用测试类型
	var testTypes []TestTypeInfo
	if mgr := h.getSelfTestManager(); mgr != nil {
		types := selftest.ListTestTypes()
		for _, t := range types {
			testTypes = append(testTypes, TestTypeInfo{
				ID:             t.ID,
				Name:           t.Name,
				Description:    t.Description,
				DefaultVariant: t.DefaultVariant,
				Variants:       t.Variants,
			})
		}
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
		},
		ChannelSources: []string{"API", "Web", "AWS", "GCP", "App"},
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
