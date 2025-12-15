package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadDotenvFromConfigDir 从配置文件所在目录加载 .env 文件。
//
// 此函数仅用于本地开发和 CLI 工具（如 verify），不应在生产服务中使用。
// 生产环境应通过 Docker/systemd 等方式注入环境变量。
//
// 行为说明：
//   - 使用 godotenv.Load：不覆盖进程中已存在的环境变量
//   - .env 文件不存在时静默忽略（返回 nil）
//   - 其它错误（权限、格式等）会返回错误
//   - verbose=true 时打印加载状态（不输出具体 key/value）
func LoadDotenvFromConfigDir(configPath string, verbose bool) error {
	if configPath == "" {
		return nil
	}

	// 获取配置文件所在目录
	configDir := filepath.Dir(configPath)
	dotenvPath := filepath.Join(configDir, ".env")

	// 尝试加载 .env 文件
	if err := godotenv.Load(dotenvPath); err != nil {
		// 检查文件是否存在
		if _, statErr := os.Stat(dotenvPath); os.IsNotExist(statErr) {
			if verbose {
				fmt.Fprintf(os.Stderr, "💡 未找到 .env，跳过加载: %s\n", dotenvPath)
			}
			return nil
		}
		// 其他错误（权限、格式等）需要报告
		return fmt.Errorf("加载 .env 失败 (%s): %w", dotenvPath, err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "✅ 已加载 .env: %s\n", dotenvPath)
	}
	return nil
}
