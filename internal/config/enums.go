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
func (s SponsorLevel) IsValid() bool {
	switch s {
	case SponsorLevelNone,
		SponsorLevelPublic, SponsorLevelSignal, SponsorLevelPulse,
		SponsorLevelBeacon, SponsorLevelBackbone, SponsorLevelCore:
		return true
	default:
		// 旧值兼容：basic/advanced/enterprise 仍视为有效
		_, ok := s.DeprecatedToNew()
		return ok
	}
}

// DeprecatedToNew 将旧赞助等级映射为新等级（向后兼容，持续 1 个版本周期）
// 返回 (新等级, 是否为旧值)
func (s SponsorLevel) DeprecatedToNew() (SponsorLevel, bool) {
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

// BadgeKind 徽标分类
type BadgeKind string

const (
	BadgeKindSource  BadgeKind = "source"  // 标识来源（如 用户Key/官方Key）
	BadgeKindInfo    BadgeKind = "info"    // 信息性提示
	BadgeKindFeature BadgeKind = "feature" // 功能特性标识
)

// IsValid 检查徽标分类是否有效
func (k BadgeKind) IsValid() bool {
	switch k {
	case BadgeKindSource, BadgeKindInfo, BadgeKindFeature:
		return true
	default:
		return false
	}
}

// BadgeVariant 徽标样式变体
type BadgeVariant string

const (
	BadgeVariantDefault BadgeVariant = "default" // 默认（中性）
	BadgeVariantSuccess BadgeVariant = "success" // 成功（正向）
	BadgeVariantWarning BadgeVariant = "warning" // 警告
	BadgeVariantDanger  BadgeVariant = "danger"  // 危险
	BadgeVariantInfo    BadgeVariant = "info"    // 信息（蓝色，社区贡献等正面信息）
)

// IsValid 检查徽标样式是否有效
func (v BadgeVariant) IsValid() bool {
	switch v {
	case BadgeVariantDefault, BadgeVariantSuccess, BadgeVariantWarning, BadgeVariantDanger, BadgeVariantInfo:
		return true
	default:
		return false
	}
}
