package main

import (
	"bufio"
	"bytes"
	"encoding/json"
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
		Timeout: 120 * time.Second, // 流式响应可能较长
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

	fmt.Println()
	fmt.Printf("📥 响应 (HTTP %d, %dms):\n", resp.StatusCode, latency.Milliseconds())

	// 检测是否为 SSE 流式响应
	// 1. 优先根据 Content-Type 判断
	// 2. 某些服务端可能未正确设置 Content-Type，使用启发式检测作为 fallback
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	isSSE := strings.Contains(contentType, "text/event-stream")

	// 使用 bufio.Reader 包装，支持 Peek 检测
	bufferedBody := bufio.NewReader(resp.Body)

	// 启发式检测：如果 Content-Type 不是 SSE，尝试 peek 开头内容
	if !isSSE {
		// Peek 可能返回 n < 512 且 err == io.EOF（短响应），仍用拿到的字节判断
		peeked, _ := bufferedBody.Peek(512)
		if len(peeked) > 0 {
			// 同时包含 "event:" 和 "data:" 视为 SSE
			if bytes.Contains(peeked, []byte("event:")) && bytes.Contains(peeked, []byte("data:")) {
				isSSE = true
				if *verbose {
					fmt.Println("ℹ️  Content-Type 未指定 SSE，但内容符合 SSE 格式")
				}
			}
		}
	}

	var respStr string
	if isSSE {
		fmt.Println("🌊 检测到流式响应 (SSE):")
		fmt.Println("────────────────────────────────")
		aggregatedText, assembledJSON, readErr := readSSEStream(bufferedBody, *verbose)
		if readErr != nil {
			fmt.Printf("\n⚠️  读取流遇到错误: %v\n", readErr)
		}
		fmt.Println()
		fmt.Println("────────────────────────────────")

		// 输出组装后的完整 JSON
		fmt.Println("\n📋 组装后的完整响应:")
		// 格式化 JSON 输出
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, []byte(assembledJSON), "", "  "); err == nil {
			fmt.Println(prettyJSON.String())
		} else {
			fmt.Println(assembledJSON)
		}

		respStr = aggregatedText
	} else {
		// 非 SSE：一次性读取
		respBody, readErr := io.ReadAll(bufferedBody)
		if readErr != nil {
			fmt.Printf("❌ 读取响应失败: %v\n", readErr)
			os.Exit(1)
		}
		respStr = string(respBody)
		if len(respStr) > 500 && !*verbose {
			fmt.Println(respStr[:500] + "...")
		} else {
			fmt.Println(respStr)
		}
		fmt.Println()
	}

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

// readSSEStream 逐行读取 SSE 流，实时输出文本内容
// 返回：(累积语义文本, 组装后的完整JSON, 错误)
func readSSEStream(r io.Reader, verbose bool) (string, string, error) {
	reader := bufio.NewReader(r)

	var (
		eventName string
		dataLines []string
		aggregate strings.Builder
		chunkNum  int

		// 用于组装完整消息的字段
		messageBase       map[string]any   // 从 message_start 获取
		contentBlocks     []map[string]any // 累积的内容块
		currentBlockIndex int              = -1
		currentBlockText  strings.Builder
		finalDelta        map[string]any // 从 message_delta 获取
	)

	// flushEvent 处理一个完整的 SSE 事件
	flushEvent := func() {
		if eventName == "" && len(dataLines) == 0 {
			return
		}

		payload := strings.Join(dataLines, "\n")

		if verbose {
			chunkNum++
			fmt.Printf("\n[chunk %d] event=%q\n", chunkNum, eventName)
			if len(payload) > 200 {
				fmt.Printf("  data: %s...\n", payload[:200])
			} else {
				fmt.Printf("  data: %s\n", payload)
			}
		}

		if payload != "" && payload != "[DONE]" {
			var obj map[string]any
			if err := json.Unmarshal([]byte(payload), &obj); err == nil {
				// 根据事件类型处理
				switch eventName {
				case "message_start":
					if msg, ok := obj["message"].(map[string]any); ok {
						messageBase = msg
					}
				case "content_block_start":
					if idx, ok := obj["index"].(float64); ok {
						currentBlockIndex = int(idx)
						currentBlockText.Reset()
					}
					if block, ok := obj["content_block"].(map[string]any); ok {
						contentBlocks = append(contentBlocks, block)
					}
				case "content_block_delta":
					if delta, ok := obj["delta"].(map[string]any); ok {
						if text, ok := delta["text"].(string); ok {
							fmt.Print(text) // 实时输出
							aggregate.WriteString(text)
							currentBlockText.WriteString(text)
							if verbose {
								fmt.Printf("  → text: %q\n", text)
							}
						}
					}
				case "content_block_stop":
					// 将累积的文本更新到对应的内容块
					if currentBlockIndex >= 0 && currentBlockIndex < len(contentBlocks) {
						contentBlocks[currentBlockIndex]["text"] = currentBlockText.String()
					}
				case "message_delta":
					finalDelta = obj
				case "response.output_text.delta":
					// OpenAI Responses API: {"delta":"pong",...}
					if text, ok := obj["delta"].(string); ok {
						fmt.Print(text) // 实时输出
						aggregate.WriteString(text)
						if verbose {
							fmt.Printf("  → text: %q\n", text)
						}
					}
				case "response.output_text.done":
					// OpenAI Responses API: {"text":"pong",...}
					if text, ok := obj["text"].(string); ok {
						// text 是完整文本，通常已经包含在增量中，这里仅作兜底
						// 如果 aggregate 为空才追加（避免重复）
						if aggregate.Len() == 0 {
							fmt.Print(text)
							aggregate.WriteString(text)
							if verbose {
								fmt.Printf("  → text (fallback): %q\n", text)
							}
						}
					}
				}
			} else {
				// 非 JSON payload，按原始文本处理
				text := payload
				fmt.Print(text)
				aggregate.WriteString(text)
				if verbose {
					fmt.Printf("  → text: %q\n", text)
				}
			}
		}

		eventName = ""
		dataLines = dataLines[:0]
	}

	for {
		line, err := reader.ReadString('\n')

		if err != nil && err != io.EOF {
			flushEvent()
			return aggregate.String(), assembleMessage(messageBase, contentBlocks, finalDelta), err
		}

		line = strings.TrimRight(line, "\r\n")

		switch {
		case line == "":
			flushEvent()
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}

		if err == io.EOF {
			flushEvent()
			return aggregate.String(), assembleMessage(messageBase, contentBlocks, finalDelta), nil
		}
	}
}

// assembleMessage 将 SSE 事件组装成完整的消息 JSON
func assembleMessage(base map[string]any, contents []map[string]any, delta map[string]any) string {
	if base == nil {
		return "{}"
	}

	// 设置内容块
	if len(contents) > 0 {
		base["content"] = contents
	}

	// 合并 message_delta 中的字段
	if delta != nil {
		if d, ok := delta["delta"].(map[string]any); ok {
			for k, v := range d {
				base[k] = v
			}
		}
		if usage, ok := delta["usage"].(map[string]any); ok {
			base["usage"] = usage
		}
	}

	result, err := json.Marshal(base)
	if err != nil {
		return "{}"
	}
	return string(result)
}
