package change

import (
	"fmt"
	"sync"

	"monitor/internal/apikey"
	"monitor/internal/config"
	"monitor/internal/logger"
	"monitor/internal/selftest"
)

// AuthIndex 运行时 API Key 指纹索引。
// 基于当前配置（已合并 env 覆盖）构建内存索引，热更新时原子重建。
type AuthIndex struct {
	mu    sync.RWMutex
	index map[string][]AuthCandidate // fingerprint → candidates
}

// NewAuthIndex 创建空索引。
func NewAuthIndex() *AuthIndex {
	return &AuthIndex{
		index: make(map[string][]AuthCandidate),
	}
}

// Rebuild 基于当前配置和 cipher 重建索引。
// monitorStore 用于判断 apply_mode（auto/manual）。
func (ai *AuthIndex) Rebuild(monitors []config.ServiceConfig, cipher *apikey.KeyCipher, monitorStore *config.MonitorStore) {
	newIndex := make(map[string][]AuthCandidate)

	for _, m := range monitors {
		// 跳过 disabled、有 parent 的子通道、无 API Key 的条目
		if m.Disabled || m.Parent != "" || m.APIKey == "" {
			continue
		}

		fingerprint := cipher.Fingerprint(m.APIKey)
		pscKey := fmt.Sprintf("%s--%s--%s", m.Provider, m.Service, m.Channel)

		applyMode := "manual"
		if monitorStore != nil {
			if mf, err := monitorStore.Get(pscKey); err == nil && mf != nil {
				applyMode = "auto"
			}
		}

		candidate := AuthCandidate{
			Provider:     m.Provider,
			Service:      m.Service,
			Channel:      m.Channel,
			MonitorKey:   pscKey,
			ApplyMode:    applyMode,
			ProviderName: m.ProviderName,
			ProviderURL:  m.ProviderURL,
			ChannelName:  m.ChannelName,
			Category:     m.Category,
			SponsorLevel: string(m.SponsorLevel),
			BaseURL:      m.BaseURL,
			KeyLast4:     apikey.Last4(m.APIKey),
		}

		// 使用 provider name 回退
		if candidate.ProviderName == "" {
			candidate.ProviderName = m.Provider
		}
		if candidate.ChannelName == "" {
			candidate.ChannelName = m.Channel
		}

		// 填充测试元数据（从 selftest 注册表查询）
		candidate.TestType = m.Service
		candidate.TestTypeName = m.Service
		if tt, ok := selftest.GetTestType(m.Service); ok {
			candidate.TestType = tt.ID
			candidate.TestTypeName = tt.Name
			candidate.DefaultTestVariant = tt.DefaultVariant
			if len(tt.Variants) > 0 {
				candidate.TestVariants = make([]TestVariant, 0, len(tt.Variants))
				for _, v := range tt.Variants {
					if v == nil {
						continue
					}
					candidate.TestVariants = append(candidate.TestVariants, TestVariant{
						ID:    v.ID,
						Order: v.Order,
					})
				}
			}
		}

		newIndex[fingerprint] = append(newIndex[fingerprint], candidate)
	}

	ai.mu.Lock()
	ai.index = newIndex
	ai.mu.Unlock()

	total := 0
	for _, cs := range newIndex {
		total += len(cs)
	}
	logger.Info("change", "API Key 认证索引已重建", "keys", len(newIndex), "candidates", total)
}

// Lookup 根据 API Key 查找匹配的通道候选。
// 返回空切片表示无匹配（不区分 key 不存在和 key 存在但无通道）。
func (ai *AuthIndex) Lookup(apiKey string, cipher *apikey.KeyCipher) []AuthCandidate {
	fingerprint := cipher.Fingerprint(apiKey)

	ai.mu.RLock()
	candidates := ai.index[fingerprint]
	ai.mu.RUnlock()

	if len(candidates) == 0 {
		return nil
	}

	// 返回深拷贝，防止外部修改内部状态
	result := make([]AuthCandidate, len(candidates))
	copy(result, candidates)
	return result
}
