package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// BadgeDef 全局徽标定义
// Label 和 Tooltip 由前端 i18n 提供，后端只存储配置元数据
type BadgeDef struct {
	ID      string       `yaml:"id" json:"id"`                   // 唯一标识（如 "api_key_user"）
	Kind    BadgeKind    `yaml:"kind" json:"kind"`               // 分类（source/info/feature）
	Variant BadgeVariant `yaml:"variant" json:"variant"`         // 样式（default/success/warning/danger/info）
	Weight  int          `yaml:"weight" json:"weight,omitempty"` // 排序权重（越大越靠前）
	URL     string       `yaml:"url" json:"url,omitempty"`       // 可选：点击跳转链接
}

// 内置默认徽标定义（无需在配置文件中定义）
// 当 monitor 未配置任何徽标时，自动注入这些默认徽标
// 注意：此变量仅供 AppConfig.Normalize() 内部使用
var defaultBadgeDefs = map[string]BadgeDef{
	"api_key_official": {
		ID:      "api_key_official",
		Kind:    BadgeKindSource,
		Variant: BadgeVariantInfo, // 蓝色，柔和
		Weight:  100,
	},
}

// BadgeRef 监测项级别的徽标引用
// 支持两种 YAML 格式：
//   - 字符串: "api_key_official"
//   - 对象: { id: "api_key_user", tooltip: "自定义提示" }
type BadgeRef struct {
	ID      string `yaml:"id" json:"id"`                                       // 引用的 BadgeDef.ID
	Tooltip string `yaml:"tooltip_override" json:"tooltip_override,omitempty"` // monitor 级 tooltip 覆盖（可选）
}

// UnmarshalYAML 支持字符串或对象两种 YAML 格式
func (r *BadgeRef) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var id string
		if err := node.Decode(&id); err != nil {
			return err
		}
		r.ID = strings.TrimSpace(id)
		r.Tooltip = ""
		return nil
	case yaml.MappingNode:
		type alias BadgeRef
		var a alias
		if err := node.Decode(&a); err != nil {
			return err
		}
		r.ID = strings.TrimSpace(a.ID)
		r.Tooltip = strings.TrimSpace(a.Tooltip)
		return nil
	default:
		return fmt.Errorf("badges 元素必须是字符串或对象")
	}
}

// BadgeProviderConfig provider 级徽标注入配置
type BadgeProviderConfig struct {
	Provider string     `yaml:"provider" json:"provider"` // provider 名称
	Badges   []BadgeRef `yaml:"badges" json:"badges"`     // 徽标引用列表
}

// ResolvedBadge 解析后的徽标（用于 API 响应）
type ResolvedBadge struct {
	ID              string       `json:"id"`
	Kind            BadgeKind    `json:"kind"`
	Variant         BadgeVariant `json:"variant"`
	Weight          int          `json:"weight,omitempty"`
	URL             string       `json:"url,omitempty"`
	TooltipOverride string       `json:"tooltip_override,omitempty"` // 仅在 monitor 级覆盖时有值
}

// RiskBadge 风险徽标配置
type RiskBadge struct {
	Label         string `yaml:"label" json:"label"`                  // 简短标签，如"跑路风险"
	DiscussionURL string `yaml:"discussion_url" json:"discussionUrl"` // 讨论页面链接（可选）
}
