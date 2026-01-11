package config

// SponsorLevel 赞助商等级
type SponsorLevel string

const (
	SponsorLevelNone       SponsorLevel = ""           // 无赞助徽章
	SponsorLevelBasic      SponsorLevel = "basic"      // 🔻 节点支持 (Node Supporter)
	SponsorLevelAdvanced   SponsorLevel = "advanced"   // ⬢ 核心服务商 (Core Provider)
	SponsorLevelEnterprise SponsorLevel = "enterprise" // 💠 全球伙伴 (Global Partner)
)

// IsValid 检查赞助商等级是否有效
func (s SponsorLevel) IsValid() bool {
	switch s {
	case SponsorLevelNone, SponsorLevelBasic, SponsorLevelAdvanced, SponsorLevelEnterprise:
		return true
	default:
		return false
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
