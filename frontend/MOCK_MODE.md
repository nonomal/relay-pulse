# 模拟数据模式使用指南

## 功能说明

Frontend 项目现已支持**双模式**运行：
- **真实 API 模式**：从后端 API 获取真实监测数据
- **模拟数据模式**：使用完全复刻 `docs/front.jsx` 的模拟数据生成器

## 如何启用模拟数据模式

### 方法 1: 环境变量配置（推荐）

在项目根目录创建 `.env.local` 文件：

```bash
VITE_USE_MOCK_DATA=true
```

### 方法 2: 修改环境配置文件

编辑 `.env.development` 或 `.env.production`：

```bash
VITE_USE_MOCK_DATA=true
```

### 方法 3: 运行时指定

```bash
VITE_USE_MOCK_DATA=true npm run dev
```

## 模拟数据特性

完全复刻 `docs/front.jsx` 的行为：
- ✅ 相同的状态分配概率（95% 可用，5-15% 波动，5% 不可用）
- ✅ 相同的延迟时间（600ms）
- ✅ 相同的历史数据生成逻辑
- ✅ 支持所有时间范围（24h, 7d, 15d, 30d）
- ✅ 刷新按钮功能正常，每次生成新的随机数据
- ✅ 视觉效果与 docs/front.jsx 完全一致

## 视觉一致性保证

- Tooltip 时间格式：`toLocaleString()` 格式化
- Tooltip 内容：仅显示时间和状态（与 docs/front.jsx 一致）
- 刷新交互：保持旧数据可见直到新数据加载完成
- 加载动画：仅在首次加载或数据为空时显示

## 开发建议

- 演示和离线开发：启用模拟数据模式
- 集成测试：使用真实 API 模式
- 生产部署：确保 `VITE_USE_MOCK_DATA=false`

## 故障排查

如果模拟数据未生效：
1. 检查 `.env.local` 文件是否存在且配置正确
2. 重启开发服务器 `npm run dev`
3. 检查控制台是否有相关日志

如果时间范围选择器无效：
- 模拟数据生成器会自动 fallback 到 24h 范围
- 检查浏览器控制台的错误日志
