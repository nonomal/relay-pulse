package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"monitor/internal/config"
	"monitor/internal/storage"
)

func main() {
	// 命令行参数
	var (
		dbURL   = flag.String("db", "", "PostgreSQL 数据库连接 URL (必需)")
		dryRun  = flag.Bool("dry-run", false, "仅检查，不执行实际迁移")
		initDB  = flag.Bool("init", false, "初始化 v1.0 数据库表结构")
		migrate = flag.Bool("migrate", false, "执行 v0 到 v1 数据迁移")
		help    = flag.Bool("help", false, "显示帮助信息")
	)
	flag.Parse()

	if *help {
		printUsage()
		os.Exit(0)
	}

	// 验证参数
	if *dbURL == "" {
		// 尝试从环境变量获取
		*dbURL = os.Getenv("DATABASE_URL")
		if *dbURL == "" {
			fmt.Fprintln(os.Stderr, "错误: 必须指定数据库连接 URL (-db 或 DATABASE_URL 环境变量)")
			printUsage()
			os.Exit(1)
		}
	}

	if !*initDB && !*migrate {
		fmt.Fprintln(os.Stderr, "错误: 必须指定至少一个操作 (-init 或 -migrate)")
		printUsage()
		os.Exit(1)
	}

	// 解析数据库 URL
	pgConfig, err := parseDBURL(*dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解析数据库 URL 失败: %v\n", err)
		os.Exit(1)
	}

	// 连接数据库
	fmt.Println("正在连接数据库...")
	store, err := storage.NewPostgresStorage(pgConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "连接数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()
	fmt.Println("数据库连接成功")

	// 执行初始化
	if *initDB {
		if err := runInit(store, *dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "初始化失败: %v\n", err)
			os.Exit(1)
		}
	}

	// 执行迁移
	if *migrate {
		if err := runMigrate(store, *dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "迁移失败: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("\n所有操作完成!")
}

func printUsage() {
	fmt.Print(`
RelayPulse v1.0 数据库迁移工具

用法:
  migrate [选项]

选项:
  -db string
        PostgreSQL 数据库连接 URL (也可通过 DATABASE_URL 环境变量设置)
        格式: postgres://user:password@host:port/dbname?sslmode=disable

  -init
        初始化 v1.0 数据库表结构
        创建 users, user_sessions, services, monitor_templates 等新表

  -migrate
        执行 v0 到 v1 数据迁移
        将 monitor_configs 数据迁移到 monitors 表

  -dry-run
        仅检查，不执行实际操作
        用于预览将要执行的变更

  -help
        显示此帮助信息

示例:
  # 初始化数据库表结构
  migrate -db "postgres://user:pass@localhost/relay?sslmode=disable" -init

  # 执行数据迁移（先预览）
  migrate -db "postgres://user:pass@localhost/relay?sslmode=disable" -migrate -dry-run

  # 执行数据迁移
  migrate -db "postgres://user:pass@localhost/relay?sslmode=disable" -migrate

  # 一次性完成初始化和迁移
  migrate -db "postgres://user:pass@localhost/relay?sslmode=disable" -init -migrate

  # 使用环境变量
  export DATABASE_URL="postgres://user:pass@localhost/relay?sslmode=disable"
  migrate -init -migrate
`)
}

// parseDBURL 解析数据库 URL 为 PostgresConfig
func parseDBURL(dbURL string) (*config.PostgresConfig, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		return nil, fmt.Errorf("无效的 URL: %w", err)
	}

	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return nil, fmt.Errorf("不支持的协议: %s (需要 postgres 或 postgresql)", u.Scheme)
	}

	cfg := &config.PostgresConfig{
		Host:     u.Hostname(),
		Database: strings.TrimPrefix(u.Path, "/"),
		SSLMode:  "disable",
	}

	// 解析端口
	if portStr := u.Port(); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("无效的端口: %s", portStr)
		}
		cfg.Port = port
	} else {
		cfg.Port = 5432
	}

	// 解析用户名和密码
	if u.User != nil {
		cfg.User = u.User.Username()
		if pwd, ok := u.User.Password(); ok {
			cfg.Password = pwd
		}
	}

	// 解析查询参数
	query := u.Query()
	if sslmode := query.Get("sslmode"); sslmode != "" {
		cfg.SSLMode = sslmode
	}

	return cfg, nil
}

func runInit(store *storage.PostgresStorage, dryRun bool) error {
	fmt.Println("\n========== 初始化 v1.0 数据库表结构 ==========")

	if dryRun {
		fmt.Println("[DRY-RUN] 将创建以下表:")
		fmt.Println("  - users (用户表)")
		fmt.Println("  - user_sessions (用户会话表)")
		fmt.Println("  - services (服务表)")
		fmt.Println("  - monitor_templates (监测模板表)")
		fmt.Println("  - monitor_template_models (模板模型表)")
		fmt.Println("  - monitors (监测项表)")
		fmt.Println("  - monitor_id_mapping (迁移映射表)")
		fmt.Println("  - monitor_applications (申请表)")
		fmt.Println("  - application_test_sessions (测试会话表)")
		fmt.Println("  - application_test_results (测试结果表)")
		fmt.Println("  - admin_audit_logs (审计日志表)")
		fmt.Println("[DRY-RUN] 跳过实际执行")
		return nil
	}

	fmt.Println("正在创建 v1.0 表结构...")
	if err := store.InitV1Tables(nil); err != nil {
		return fmt.Errorf("创建表结构失败: %w", err)
	}

	fmt.Println("v1.0 表结构创建成功!")
	return nil
}

func runMigrate(store *storage.PostgresStorage, dryRun bool) error {
	fmt.Println("\n========== 执行 v0 到 v1 数据迁移 ==========")

	if dryRun {
		fmt.Println("[DRY-RUN] 将执行以下迁移:")
		fmt.Println("  1. 从 monitor_configs 读取现有监测项配置")
		fmt.Println("  2. 将配置转换为 monitors 表格式")
		fmt.Println("  3. 创建 monitor_id_mapping 记录")
		fmt.Println("  4. 迁移 monitor_secrets 关联")
		fmt.Println("[DRY-RUN] 跳过实际执行")
		return nil
	}

	fmt.Println("正在执行数据迁移...")
	startTime := time.Now()

	stats, err := store.MigrateFromV0WithStats(nil)
	if err != nil {
		return fmt.Errorf("数据迁移失败: %w", err)
	}

	duration := time.Since(startTime)

	fmt.Printf("\n迁移完成! (耗时: %v)\n", duration.Round(time.Millisecond))
	fmt.Printf("  - 总计: %d 条\n", stats.Total)
	fmt.Printf("  - 迁移成功: %d 条\n", stats.Migrated)
	fmt.Printf("  - 跳过(已存在): %d 条\n", stats.Skipped)
	if stats.Failed > 0 {
		fmt.Printf("  - 失败: %d 条\n", stats.Failed)
		for _, e := range stats.Errors {
			fmt.Printf("    - %s\n", e)
		}
	}

	return nil
}
