package config

// SponsorLevel 赞助等级（通道级）
type SponsorLevel string

const (
	SponsorLevelNone     SponsorLevel = ""         // 无赞助标识
	SponsorLevelPublic   SponsorLevel = "public"   // 🛡️ 公益链路 (Public)
	SponsorLevelSignal   SponsorLevel = "signal"   // · 信号链路 (Signal)
	SponsorLevelPulse    SponsorLevel = "pulse"    // ◆ 脉冲链路 (Pulse)
	SponsorLevelBeacon   SponsorLevel = "beacon"   // 🔺 信标链路 (Beacon)
	SponsorLevelBackbone SponsorLevel = "backbone" // ⬢ 骨干链路 (Backbone)
	SponsorLevelCore     SponsorLevel = "core"     // 💠 核心链路 (Core)
)

// IsValid 检查赞助等级是否有效（含旧值兼容）
func (s SponsorLevel) isValid() bool {
	switch s {
	case SponsorLevelNone,
		SponsorLevelPublic, SponsorLevelSignal, SponsorLevelPulse,
		SponsorLevelBeacon, SponsorLevelBackbone, SponsorLevelCore:
		return true
	default:
		// 旧值兼容：basic/advanced/enterprise 仍视为有效
		_, ok := s.deprecatedToNew()
		return ok
	}
}

// DeprecatedToNew 将旧赞助等级映射为新等级（向后兼容，持续 1 个版本周期）
// 返回 (新等级, 是否为旧值)
func (s SponsorLevel) deprecatedToNew() (SponsorLevel, bool) {
	switch s {
	case "basic":
		return SponsorLevelPulse, true
	case "advanced":
		return SponsorLevelBackbone, true
	case "enterprise":
		return SponsorLevelCore, true
	default:
		return s, false
	}
}

// Weight 返回赞助等级的权重（用于排序和比较）
func (s SponsorLevel) Weight() int {
	switch s {
	case SponsorLevelCore:
		return 6
	case SponsorLevelBackbone:
		return 5
	case SponsorLevelBeacon:
		return 4
	case SponsorLevelPulse:
		return 3
	case SponsorLevelSignal:
		return 2
	case SponsorLevelPublic:
		return 1
	default:
		return 0
	}
}
