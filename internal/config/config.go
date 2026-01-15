// Package config 提供应用配置管理功能
//
// 主要职责：
//   - 配置结构体定义（AppConfig、ServiceConfig 等）
//   - 配置验证（Validate）
//   - 配置规范化（Normalize）
//   - 环境变量覆盖（ApplyEnvOverrides）
//   - 配置热更新支持（Clone、Watch）
//
// 文件组织：
//   - app_config.go: AppConfig 结构体定义
//   - validate.go: 配置验证逻辑
//   - normalize.go: 配置规范化入口和全局函数
//   - normalize_monitors.go: 监测项规范化
//   - parent_inheritance.go: 父子继承逻辑
//   - lifecycle.go: 生命周期方法（Clone、ApplyEnvOverrides 等）
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
