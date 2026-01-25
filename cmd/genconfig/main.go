package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"monitor/cmd/genconfig/generator"
)

func main() {
	mode := flag.String("mode", "interactive", "生成模式: interactive(交互式) 或 template(模板快速生成)")
	template := flag.String("template", "", "模板名称 (仅在 mode=template 时使用)")
	output := flag.String("output", "", "输出文件路径 (不指定则输出到 stdout)")
	appendMode := flag.Bool("append", false, "追加到现有配置文件 (monitors-only，仅在指定 output 时生效)")
	listTemplates := flag.Bool("list", false, "列出所有可用模板")

	flag.Parse()

	// 列出模板
	if *listTemplates {
		registry := generator.NewTemplateRegistry()
		fmt.Println("📋 可用模板:")
		for _, name := range registry.ListTemplates() {
			fmt.Printf("  - %s\n", name)
		}
		fmt.Println("\n使用方式: go run ./cmd/genconfig -mode template -template <name>")
		return
	}

	var config string
	var err error

	switch *mode {
	case "interactive":
		config, err = runInteractiveMode()
	case "template":
		if *template == "" {
			fmt.Println("❌ 模板模式需要指定 -template 参数")
			fmt.Println("使用 -list 查看所有可用模板")
			os.Exit(1)
		}
		config, err = generator.GenerateFromTemplate(*template)
	default:
		fmt.Printf("❌ 未知的模式: %s\n", *mode)
		os.Exit(1)
	}

	if err != nil {
		fmt.Printf("❌ 生成配置失败: %v\n", err)
		os.Exit(1)
	}

	// 输出配置
	if *output == "" {
		fmt.Println(config)
	} else {
		err := generator.WriteConfig(config, *output, *appendMode)
		if err != nil {
			fmt.Printf("❌ 写入文件失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ 配置已保存到: %s\n", *output)
	}
}

func runInteractiveMode() (string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n🚀 RelayPulse 配置生成器 - 交互式模式")
	fmt.Println(strings.Repeat("=", 50))

	// 收集全局配置
	fmt.Println("\n📋 全局配置")
	interval := promptWithDefault(reader, "巡检间隔 (Go duration 格式)", "1m")
	slowLatency := promptWithDefault(reader, "慢请求阈值", "5s")
	timeout := promptWithDefault(reader, "请求超时时间", "10s")

	// 收集监测项
	fmt.Println("\n📝 监测项配置")
	monitors := []map[string]string{}

	for {
		fmt.Printf("\n--- 监测项 #%d ---\n", len(monitors)+1)

		provider := prompt(reader, "服务商标识 (provider)")
		service := promptEnum(reader, "服务类型 (service)", []string{"cc", "cx", "gm"})
		category := promptEnumWithDefault(reader, "分类 (commercial/public)", "commercial", []string{"commercial", "public"})
		sponsor := prompt(reader, "赞助者名称 (sponsor)")
		channel := promptWithDefault(reader, "业务通道 (channel)", service)
		board := promptEnumWithDefault(reader, "板块 (hot/secondary/cold)", "hot", []string{"hot", "secondary", "cold"})
		url := prompt(reader, "健康检查端点 URL")
		method := promptEnumWithDefault(reader, "HTTP 方法", "POST", []string{"GET", "POST", "PUT", "DELETE", "PATCH"})
		successContains := promptWithDefault(reader, "响应体关键字 (success_contains)", "")

		monitor := map[string]string{
			"provider":         provider,
			"service":          service,
			"category":         category,
			"sponsor":          sponsor,
			"channel":          channel,
			"board":            board,
			"url":              url,
			"method":           method,
			"success_contains": successContains,
		}
		monitors = append(monitors, monitor)

		addMore := promptWithDefault(reader, "继续添加监测项? (y/n)", "n")
		if strings.ToLower(addMore) != "y" {
			break
		}
	}

	// 生成配置
	return generator.GenerateConfig(interval, slowLatency, timeout, monitors)
}

func prompt(reader *bufio.Reader, label string) string {
	for {
		fmt.Printf("%s: ", label)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			return input
		}
		fmt.Println("❌ 不能为空，请重新输入")
	}
}

func promptWithDefault(reader *bufio.Reader, label, defaultValue string) string {
	fmt.Printf("%s [%s]: ", label, defaultValue)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}

func promptEnum(reader *bufio.Reader, label string, allowed []string) string {
	for {
		fmt.Printf("%s (%s): ", label, strings.Join(allowed, "/"))
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			fmt.Println("❌ 不能为空，请重新输入")
			continue
		}
		// 对于 service/board/category，做大小写归一化后再比对
		inputLower := strings.ToLower(input)
		for _, a := range allowed {
			if inputLower == strings.ToLower(a) {
				return a // 返回标准值
			}
		}
		fmt.Printf("❌ 无效值: %s（支持: %s）\n", input, strings.Join(allowed, "/"))
	}
}

func promptEnumWithDefault(reader *bufio.Reader, label, defaultValue string, allowed []string) string {
	for {
		fmt.Printf("%s [%s] (%s): ", label, defaultValue, strings.Join(allowed, "/"))
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return defaultValue
		}
		// 对于 service/board/category，做大小写归一化后再比对
		inputLower := strings.ToLower(input)
		for _, a := range allowed {
			if inputLower == strings.ToLower(a) {
				return a // 返回标准值
			}
		}
		fmt.Printf("❌ 无效值: %s（支持: %s）\n", input, strings.Join(allowed, "/"))
	}
}
