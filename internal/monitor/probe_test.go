package monitor

import (
	"testing"

	"monitor/internal/storage"
)

func TestEvaluateStatusWithoutSuccessContains(t *testing.T) {
	t.Parallel()

	status, subStatus := evaluateStatus(1, storage.SubStatusNone, []byte("anything"), "")
	if status != 1 {
		t.Fatalf("expected status 1 when success_contains is empty, got %d", status)
	}
	if subStatus != storage.SubStatusNone {
		t.Fatalf("expected SubStatusNone, got %s", subStatus)
	}
}

func TestEvaluateStatusWithMatchingContent(t *testing.T) {
	t.Parallel()

	body := []byte(`{"ok":true,"message":"pong"}`)
	status, subStatus := evaluateStatus(1, storage.SubStatusNone, body, "pong")
	if status != 1 {
		t.Fatalf("expected status 1 when body contains keyword, got %d", status)
	}
	if subStatus != storage.SubStatusNone {
		t.Fatalf("expected SubStatusNone, got %s", subStatus)
	}
}

func TestEvaluateStatusWithNonMatchingContent(t *testing.T) {
	t.Parallel()

	body := []byte(`{"ok":false,"message":"error"}`)
	status, subStatus := evaluateStatus(1, storage.SubStatusNone, body, "pong")
	if status != 0 {
		t.Fatalf("expected status 0 when body does not contain keyword, got %d", status)
	}
	if subStatus != storage.SubStatusContentMismatch {
		t.Fatalf("expected SubStatusContentMismatch, got %s", subStatus)
	}
}

func TestEvaluateStatusWithStreamingContentSplit(t *testing.T) {
	t.Parallel()

	// 模拟 SSE 流式增量：先返回 "p"，再返回 "ong"
	body := []byte(
		"event: content_block_delta\n" +
			"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"p\"}}\n\n" +
			"event: content_block_delta\n" +
			"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"ong\"}}\n\n",
	)

	status, subStatus := evaluateStatus(1, storage.SubStatusNone, body, "pong")
	if status != 1 {
		t.Fatalf("expected status 1 for streaming body containing aggregated keyword, got %d", status)
	}
	if subStatus != storage.SubStatusNone {
		t.Fatalf("expected SubStatusNone for streaming body, got %s", subStatus)
	}
}

func TestEvaluateStatusWithGeminiSSE(t *testing.T) {
	t.Parallel()

	// 模拟 Gemini SSE 格式：没有 event: 行，只有 data: 行
	// 流式响应中 text 被拆分到多个 chunk
	body := []byte(
		`data: {"candidates":[{"content":{"parts":[{"text":"po"}],"role":"model"},"index":0}]}` + "\n\n" +
			`data: {"candidates":[{"content":{"parts":[{"text":"ng"}],"role":"model"},"index":0}]}` + "\n\n",
	)

	status, subStatus := evaluateStatus(1, storage.SubStatusNone, body, "pong")
	if status != 1 {
		t.Fatalf("expected status 1 for Gemini SSE body containing aggregated keyword, got %d", status)
	}
	if subStatus != storage.SubStatusNone {
		t.Fatalf("expected SubStatusNone for Gemini SSE body, got %s", subStatus)
	}
}

func TestEvaluateStatusWithGeminiSSENoEventLine(t *testing.T) {
	t.Parallel()

	// 完整的 Gemini 响应示例（只有 data: 行，文本完整在一个 chunk）
	body := []byte(
		`data: {"candidates":[{"content":{"parts":[{"text":"pong"}],"role":"model"},"index":0}],"usageMetadata":{"promptTokenCount":8}}` + "\n\n" +
			`data: {"candidates":[{"content":{"parts":[{"text":"","thoughtSignature":"xxx"}],"role":"model"},"finishReason":"MAX_TOKENS","index":0}]}` + "\n\n",
	)

	status, subStatus := evaluateStatus(1, storage.SubStatusNone, body, "pong")
	if status != 1 {
		t.Fatalf("expected status 1 for Gemini SSE with complete text, got %d", status)
	}
	if subStatus != storage.SubStatusNone {
		t.Fatalf("expected SubStatusNone, got %s", subStatus)
	}
}
