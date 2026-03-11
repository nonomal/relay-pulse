// Package config 提供应用配置管理功能
//
// 主要职责：
//   - 配置结构体定义（AppConfig、ServiceConfig 等）
//   - 配置加载、验证与规范化
//   - 环境变量覆盖、模板解析与热更新
//
// 文件组织：
//   - app_config.go: AppConfig 结构体定义
//   - validate.go: 配置验证逻辑
//   - normalize.go: 配置规范化逻辑
//   - normalize_monitors.go: 监测项规范化
//   - parent_inheritance.go: 父子继承逻辑
//   - lifecycle.go: 生命周期内部步骤与运行时辅助
//   - loader.go: 配置加载（YAML 解析）
//   - watcher.go: 配置热更新监听
//   - monitor.go: ServiceConfig 定义
//   - helpers.go: 辅助函数
//   - enums.go: 枚举类型定义
//   - badges.go: 徽标相关类型
//   - features.go: 功能模块配置类型
//   - storage_config.go: 存储配置类型
//   - external.go: 外部服务配置类型
package config
