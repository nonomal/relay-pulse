package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"monitor/internal/config"
)

func main() {
	provider := flag.String("provider", "", "Provider name (required)")
	service := flag.String("service", "", "Service name (required)")
	channel := flag.String("channel", "", "Channel name (optional, defaults to service)")
	configFile := flag.String("config", "config.yaml", "Config file path")
	verbose := flag.Bool("v", false, "Verbose output")

	flag.Parse()

	if *provider == "" || *service == "" {
		fmt.Println("用法: go run cmd/verify/main.go -provider <name> -service <name> [-channel <name>] [-config <path>] [-v]")
		fmt.Println("示例: go run cmd/verify/main.go -provider AICodeMirror -service cc -v")
		os.Exit(1)
	}

	if *channel == "" {
		*channel = *service
	}

	// 加载 .env 文件（仅用于本地开发，不覆盖已有环境变量）
	if err := config.LoadDotenvFromConfigDir(*configFile, *verbose); err != nil {
		fmt.Printf("⚠️  %v\n", err)
		// 不中断执行，继续尝试加载配置
	}

	// 加载配置
	loader := config.NewLoader()
	cfg, err := loader.Load(*configFile)
	if err != nil {
		fmt.Printf("❌ 加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 查找检测项
	var target *config.ServiceConfig
	for i := range cfg.Monitors {
		m := &cfg.Monitors[i]
		if m.Provider == *provider && m.Service == *service {
			if *channel == "" || m.Channel == *channel {
				target = m
				break
			}
		}
	}

	if target == nil {
		fmt.Printf("❌ 未找到检测项: provider=%s, service=%s, channel=%s\n", *provider, *service, *channel)
		os.Exit(1)
	}

	fmt.Printf("🔍 验证检测项: provider=%s, service=%s, channel=%s\n", target.Provider, target.Service, target.Channel)
	fmt.Println("========================================")

	if *verbose {
		fmt.Printf("📋 配置信息:\n")
		fmt.Printf("  URL: %s\n", target.URL)
		fmt.Printf("  Method: %s\n", target.Method)
		fmt.Printf("  Success Contains: %s\n", target.SuccessContains)
		fmt.Printf("  Headers:\n")
		for k, v := range target.Headers {
			// 隐藏 API key
			if strings.Contains(strings.ToLower(k), "key") || strings.Contains(strings.ToLower(k), "auth") {
				v = v[:min(10, len(v))] + "..."
			}
			fmt.Printf("    %s: %s\n", k, v)
		}
		fmt.Printf("  Body (%d bytes):\n", len(target.Body))
		if len(target.Body) > 200 {
			fmt.Printf("    %s...\n", target.Body[:200])
		} else {
			fmt.Printf("    %s\n", target.Body)
		}
		fmt.Println()
	}

	// 构建请求
	var body io.Reader
	if target.Body != "" {
		// 去除首尾空白字符（某些 API 对此敏感）
		trimmedBody := strings.TrimSpace(target.Body)
		body = bytes.NewBufferString(trimmedBody)
	}

	req, err := http.NewRequest(target.Method, target.URL, body)
	if err != nil {
		fmt.Printf("❌ 构建请求失败: %v\n", err)
		os.Exit(1)
	}

	// 设置 headers（使用原始大小写）
	for k, v := range target.Headers {
		// 直接操作 map 以保持原始大小写
		req.Header[k] = []string{v}
	}

	// 打印实际请求 headers
	if *verbose {
		fmt.Println("📨 实际请求 Headers:")
		for k, v := range req.Header {
			fmt.Printf("    %s: %s\n", k, v)
		}
		fmt.Println()
	}

	fmt.Println("📤 发送请求...")
	start := time.Now()

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy:              http.ProxyFromEnvironment,
			DisableCompression: true, // 禁用自动解压缩
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	latency := time.Since(start)

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ 读取响应失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("📥 响应 (HTTP %d, %dms):\n", resp.StatusCode, latency.Milliseconds())

	// 截断显示
	respStr := string(respBody)
	if len(respStr) > 500 && !*verbose {
		fmt.Println(respStr[:500] + "...")
	} else {
		fmt.Println(respStr)
	}
	fmt.Println()

	// 判断结果
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if target.SuccessContains != "" {
			if strings.Contains(respStr, target.SuccessContains) {
				fmt.Printf("✅ 成功! HTTP %d, 延迟 %dms, 响应包含 '%s'\n", resp.StatusCode, latency.Milliseconds(), target.SuccessContains)
			} else {
				fmt.Printf("⚠️  HTTP %d 但响应不包含 '%s'\n", resp.StatusCode, target.SuccessContains)
				os.Exit(1)
			}
		} else {
			fmt.Printf("✅ 成功! HTTP %d, 延迟 %dms\n", resp.StatusCode, latency.Milliseconds())
		}
	} else {
		fmt.Printf("❌ 失败! HTTP %d, 延迟 %dms\n", resp.StatusCode, latency.Milliseconds())
		os.Exit(1)
	}
}
