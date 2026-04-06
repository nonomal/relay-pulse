package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"monitor/internal/config"
	"monitor/internal/logger"
)

// AdminListTemplates 列出 templates/ 中的可用模板
// GET /api/admin/templates
func (h *Handler) AdminListTemplates(c *gin.Context) {
	if !h.checkAdminToken(c) {
		return
	}

	store := h.getMonitorStore()
	if store == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "monitors.d 管理未启用")
		return
	}

	templatesDir := filepath.Join(filepath.Dir(store.Dir()), "templates")
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"templates": []string{}})
			return
		}
		logger.Error("admin", "读取模板目录失败", "dir", templatesDir, "error", err)
		apiError(c, http.StatusInternalServerError, ErrCodeInternalError, "读取模板目录失败")
		return
	}

	templates := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		templates = append(templates, strings.TrimSuffix(name, ".json"))
	}
	sort.Strings(templates)

	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

// AdminListMonitors 列出所有 monitors.d/ 中的监测项
// GET /api/admin/monitors
func (h *Handler) AdminListMonitors(c *gin.Context) {
	if !h.checkAdminToken(c) {
		return
	}

	store := h.getMonitorStore()
	if store == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "monitors.d 管理未启用")
		return
	}

	summaries, err := store.List()
	if err != nil {
		logger.Error("admin", "列出监测项失败", "error", err)
		apiError(c, http.StatusInternalServerError, ErrCodeInternalError, "列出监测项失败")
		return
	}

	// 过滤
	board := strings.TrimSpace(c.Query("board"))
	status := strings.TrimSpace(c.Query("status"))
	query := strings.ToLower(strings.TrimSpace(c.Query("q")))

	var filtered []config.MonitorSummary
	for _, s := range summaries {
		if board != "" && s.Board != board {
			continue
		}
		if status == "disabled" && !s.Disabled {
			continue
		}
		if status == "hidden" && !s.Hidden {
			continue
		}
		if status == "active" && (s.Disabled || s.Hidden) {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(s.Provider + " " + s.Service + " " + s.Channel + " " + s.Template)
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		filtered = append(filtered, s)
	}

	c.JSON(http.StatusOK, gin.H{
		"monitors": filtered,
		"total":    len(filtered),
	})
}

// AdminGetMonitor 获取指定监测项详情
// GET /api/admin/monitors/:key
func (h *Handler) AdminGetMonitor(c *gin.Context) {
	if !h.checkAdminToken(c) {
		return
	}

	store := h.getMonitorStore()
	if store == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "monitors.d 管理未启用")
		return
	}

	key := c.Param("key")
	file, err := store.Get(key)
	if err != nil {
		logger.Error("admin", "获取监测项失败", "key", key, "error", err)
		apiError(c, http.StatusInternalServerError, ErrCodeInternalError, "获取监测项失败")
		return
	}
	if file == nil {
		apiError(c, http.StatusNotFound, ErrCodeNotFound, "监测项不存在")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"monitor": file,
	})
}

// AdminCreateMonitor 创建新监测项
// POST /api/admin/monitors
func (h *Handler) AdminCreateMonitor(c *gin.Context) {
	if !h.checkAdminToken(c) {
		return
	}

	store := h.getMonitorStore()
	if store == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "monitors.d 管理未启用")
		return
	}

	var file config.MonitorFile
	if err := c.ShouldBindJSON(&file); err != nil {
		apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "请求参数无效")
		return
	}

	if len(file.Monitors) == 0 {
		apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "monitors 不能为空")
		return
	}

	// 验证基本字段
	for i, m := range file.Monitors {
		if strings.TrimSpace(m.Provider) == "" && strings.TrimSpace(m.Parent) == "" {
			apiError(c, http.StatusBadRequest, ErrCodeInvalidParam,
				"monitors["+string(rune('0'+i))+"]: provider 不能为空（或通过 parent 继承）")
			return
		}
	}

	if file.Metadata.Source == "" {
		file.Metadata.Source = "admin"
	}

	// 跨源 PSC 冲突预检：确保新 PSC 不与 config.yaml 中已有的监测项冲突
	if err := h.checkPSCConflict(&file); err != nil {
		apiError(c, http.StatusConflict, ErrCodeInvalidParam, err.Error())
		return
	}

	if err := store.Create(&file); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "已存在") {
			apiError(c, http.StatusConflict, ErrCodeInvalidParam, errMsg)
			return
		}
		if strings.Contains(errMsg, "无效") || strings.Contains(errMsg, "不能") {
			apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, errMsg)
			return
		}
		logger.Error("admin", "创建监测项失败", "error", err)
		apiError(c, http.StatusInternalServerError, ErrCodeInternalError, errMsg)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"monitor": file,
	})
}

// AdminUpdateMonitor 更新监测项
// PUT /api/admin/monitors/:key
func (h *Handler) AdminUpdateMonitor(c *gin.Context) {
	if !h.checkAdminToken(c) {
		return
	}

	store := h.getMonitorStore()
	if store == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "monitors.d 管理未启用")
		return
	}

	key := c.Param("key")

	var req struct {
		Revision int64              `json:"revision"`
		Monitor  config.MonitorFile `json:"monitor"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "请求参数无效")
		return
	}

	if len(req.Monitor.Monitors) == 0 {
		apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "monitors 不能为空")
		return
	}

	if err := store.Update(key, &req.Monitor, req.Revision); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "不存在") {
			apiError(c, http.StatusNotFound, ErrCodeNotFound, errMsg)
			return
		}
		if strings.Contains(errMsg, "revision") {
			apiError(c, http.StatusConflict, ErrCodeInvalidParam, errMsg)
			return
		}
		if strings.Contains(errMsg, "不可变更") || strings.Contains(errMsg, "无效") {
			apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, errMsg)
			return
		}
		logger.Error("admin", "更新监测项失败", "key", key, "error", err)
		apiError(c, http.StatusInternalServerError, ErrCodeInternalError, errMsg)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"monitor": req.Monitor,
	})
}

// AdminDeleteMonitor 归档删除监测项
// DELETE /api/admin/monitors/:key
func (h *Handler) AdminDeleteMonitor(c *gin.Context) {
	if !h.checkAdminToken(c) {
		return
	}

	store := h.getMonitorStore()
	if store == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "monitors.d 管理未启用")
		return
	}

	key := c.Param("key")
	if err := store.Delete(key); err != nil {
		if strings.Contains(err.Error(), "不存在") {
			apiError(c, http.StatusNotFound, ErrCodeNotFound, err.Error())
			return
		}
		logger.Error("admin", "删除监测项失败", "key", key, "error", err)
		apiError(c, http.StatusInternalServerError, ErrCodeInternalError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "archived"})
}

// AdminToggleMonitor 切换监测项的 disabled/hidden 状态
// POST /api/admin/monitors/:key/toggle
func (h *Handler) AdminToggleMonitor(c *gin.Context) {
	if !h.checkAdminToken(c) {
		return
	}

	store := h.getMonitorStore()
	if store == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "monitors.d 管理未启用")
		return
	}

	key := c.Param("key")

	var req struct {
		Field string `json:"field" binding:"required"` // "disabled" or "hidden"
		Value bool   `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "请求参数无效")
		return
	}
	if req.Field != "disabled" && req.Field != "hidden" {
		apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "field 只能是 disabled 或 hidden")
		return
	}

	file, err := store.Get(key)
	if err != nil {
		logger.Error("admin", "获取监测项失败", "key", key, "error", err)
		apiError(c, http.StatusInternalServerError, ErrCodeInternalError, "获取监测项失败")
		return
	}
	if file == nil {
		apiError(c, http.StatusNotFound, ErrCodeNotFound, "监测项不存在")
		return
	}

	for i := range file.Monitors {
		if strings.TrimSpace(file.Monitors[i].Parent) != "" {
			continue // 只修改父通道
		}
		switch req.Field {
		case "disabled":
			file.Monitors[i].Disabled = req.Value
		case "hidden":
			file.Monitors[i].Hidden = req.Value
		}
	}

	if err := store.Update(key, file, file.Metadata.Revision); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "revision") {
			apiError(c, http.StatusConflict, ErrCodeInvalidParam, errMsg)
			return
		}
		logger.Error("admin", "切换监测项状态失败", "key", key, "error", err)
		apiError(c, http.StatusInternalServerError, ErrCodeInternalError, errMsg)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"monitor": file,
	})
}

// AdminProbeMonitor 对监测项执行探测测试
// POST /api/admin/monitors/:key/probe
func (h *Handler) AdminProbeMonitor(c *gin.Context) {
	if !h.checkAdminToken(c) {
		return
	}

	store := h.getMonitorStore()
	if store == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "monitors.d 管理未启用")
		return
	}

	if h.inlineProber == nil {
		apiError(c, http.StatusServiceUnavailable, ErrCodeFeatureDisabled, "内联探测器未初始化")
		return
	}

	key := c.Param("key")
	file, err := store.Get(key)
	if err != nil {
		logger.Error("admin", "获取监测项失败", "key", key, "error", err)
		apiError(c, http.StatusInternalServerError, ErrCodeInternalError, "获取监测项失败")
		return
	}
	if file == nil {
		apiError(c, http.StatusNotFound, ErrCodeNotFound, "监测项不存在")
		return
	}

	// 找到父通道
	var root *config.ServiceConfig
	for i := range file.Monitors {
		if strings.TrimSpace(file.Monitors[i].Parent) == "" {
			root = &file.Monitors[i]
			break
		}
	}
	if root == nil {
		apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "找不到父通道")
		return
	}

	if root.BaseURL == "" {
		apiError(c, http.StatusBadRequest, ErrCodeInvalidParam, "base_url 未配置")
		return
	}

	// 使用内联探测器同步执行
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result := h.inlineProber.Probe(ctx, root.Service, root.Template, root.BaseURL, root.APIKey)

	c.JSON(http.StatusOK, gin.H{
		"probe_id":         result.ProbeID,
		"probe_status":     result.ProbeStatus,
		"sub_status":       result.SubStatus,
		"http_code":        result.HTTPCode,
		"latency":          result.Latency,
		"error_message":    result.ErrorMessage,
		"response_snippet": result.ResponseSnippet,
	})
}

// checkPSCConflict 预检新 MonitorFile 的 PSC 是否与当前已加载配置冲突。
// 检查范围：config.yaml 中已加载的监测项。
// monitors.d/ 内部冲突由 MonitorStore.Create 的文件系统检查覆盖。
func (h *Handler) checkPSCConflict(file *config.MonitorFile) error {
	pscKey, err := config.DeriveMonitorFileKey(*file)
	if err != nil {
		return err
	}

	// 将 monitors.d/ key（provider--service--channel）转为 PSC 格式（provider/service/channel）
	p, s, c, err := config.ParseMonitorFileKey(pscKey)
	if err != nil {
		return err
	}
	target := strings.ToLower(p) + "/" + strings.ToLower(s) + "/" + strings.ToLower(c)

	h.cfgMu.RLock()
	currentMonitors := h.config.Monitors
	h.cfgMu.RUnlock()

	existingKeys := config.CollectPSCKeys(currentMonitors)
	if _, exists := existingKeys[target]; exists {
		return &pscConflictError{psc: target}
	}
	return nil
}

type pscConflictError struct {
	psc string
}

func (e *pscConflictError) Error() string {
	return "PSC " + e.psc + " 已存在于当前配置中（config.yaml）"
}
