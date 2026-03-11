package notifier

import (
	"slices"
	"testing"

	"notifier/internal/poller"
)

// 固定时间戳：2024-01-01 00:00:00 UTC = 2024-01-01 08:00:00 CST
const (
	fixedEventTimestamp int64  = 1704067200
	fixedEventTimeCST   string = "2024-01-01 08:00:00"
)

func TestExtractModels(t *testing.T) {
	tests := []struct {
		name    string
		event   *poller.Event
		want    []string
		wantNil bool
	}{
		{
			name:    "nil 事件",
			event:   nil,
			wantNil: true,
		},
		{
			name:    "空 Meta 无 Model",
			event:   &poller.Event{Meta: map[string]any{}},
			wantNil: true,
		},
		{
			name: "Meta models 为 []string",
			event: &poller.Event{
				Meta: map[string]any{
					"models": []string{" o1 ", "", "gpt-4o"},
				},
			},
			want: []string{"gpt-4o", "o1"},
		},
		{
			name: "Meta models 为 []any 含非 string",
			event: &poller.Event{
				Meta: map[string]any{
					"models": []any{" o1 ", 42, "", "gpt-4o"},
				},
			},
			want: []string{"gpt-4o", "o1"},
		},
		{
			name: "回退 event.Model",
			event: &poller.Event{
				Model: " claude-3.5-sonnet ",
			},
			want: []string{"claude-3.5-sonnet"},
		},
		{
			name: "去重并排序",
			event: &poller.Event{
				Model: "gpt-4o",
				Meta: map[string]any{
					"models": []string{" o1 ", "claude-3.5-sonnet", "gpt-4o", "o1"},
				},
			},
			want: []string{"claude-3.5-sonnet", "gpt-4o", "o1"},
		},
		{
			name: "Meta 有 models 键但类型不支持时回退 Model",
			event: &poller.Event{
				Model: "fallback-model",
				Meta: map[string]any{
					"models": 12345, // 不支持的类型
				},
			},
			want: []string{"fallback-model"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractModels(tt.event)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("extractModels() = %#v, want nil", got)
				}
				return
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("extractModels() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// messageFormatCase 共享 Telegram/QQ 格式化测试用例
type messageFormatCase struct {
	name         string
	event        *poller.Event
	wantTelegram string
	wantQQ       string
}

func messageFormatCases() []messageFormatCase {
	return []messageFormatCase{
		{
			name: "UP 含 channel 和单模型",
			event: &poller.Event{
				Provider:   "openai",
				Service:    "chatgpt",
				Channel:    "web",
				Model:      "gpt-4o",
				Type:       "UP",
				ObservedAt: fixedEventTimestamp,
			},
			wantTelegram: "🟢 <b>服务已恢复</b>\n\n" +
				"<b>openai</b> / <b>chatgpt</b> / <b>web</b>\n" +
				"模型: gpt-4o\n\n" +
				"时间: " + fixedEventTimeCST,
			wantQQ: "🟢 服务已恢复\n\n" +
				"openai / chatgpt / web\n" +
				"模型: gpt-4o\n\n" +
				"时间: " + fixedEventTimeCST,
		},
		{
			name: "DOWN 无 channel 含 sub_status 且 HTML 转义",
			event: &poller.Event{
				Provider:   "<Open&AI>",
				Service:    "status<prod>&v1",
				Model:      "gpt-4 <mini>&beta",
				Type:       "DOWN",
				ObservedAt: fixedEventTimestamp,
				Meta: map[string]any{
					"sub_status": "<bad & slow>",
				},
			},
			wantTelegram: "🔴 <b>服务不可用</b>\n\n" +
				"<b>&lt;Open&amp;AI&gt;</b> / <b>status&lt;prod&gt;&amp;v1</b>\n" +
				"模型: gpt-4 &lt;mini&gt;&amp;beta\n" +
				"原因: &lt;bad &amp; slow&gt;\n\n" +
				"时间: " + fixedEventTimeCST,
			wantQQ: "🔴 服务不可用\n\n" +
				"<Open&AI> / status<prod>&v1\n" +
				"模型: gpt-4 <mini>&beta\n" +
				"原因: <bad & slow>\n\n" +
				"时间: " + fixedEventTimeCST,
		},
		{
			name: "无 Type 回退 ToStatus=2 波动 + 多模型",
			event: &poller.Event{
				Provider:   "anthropic",
				Service:    "messages",
				ToStatus:   2,
				ObservedAt: fixedEventTimestamp,
				Meta: map[string]any{
					"models": []any{" o1 ", 7, "gpt-4o", "gpt-4o"},
				},
			},
			wantTelegram: "🟡 <b>服务波动</b>\n\n" +
				"<b>anthropic</b> / <b>messages</b>\n" +
				"模型: gpt-4o, o1\n\n" +
				"时间: " + fixedEventTimeCST,
			wantQQ: "🟡 服务波动\n\n" +
				"anthropic / messages\n" +
				"模型: gpt-4o, o1\n\n" +
				"时间: " + fixedEventTimeCST,
		},
		{
			name: "无 Type ToStatus=0 不可用",
			event: &poller.Event{
				Provider:   "azure",
				Service:    "openai",
				ToStatus:   0,
				ObservedAt: fixedEventTimestamp,
			},
			wantTelegram: "🔴 <b>服务不可用</b>\n\n" +
				"<b>azure</b> / <b>openai</b>\n\n" +
				"时间: " + fixedEventTimeCST,
			wantQQ: "🔴 服务不可用\n\n" +
				"azure / openai\n\n" +
				"时间: " + fixedEventTimeCST,
		},
		{
			name: "无 Type ToStatus=1 恢复",
			event: &poller.Event{
				Provider:   "test",
				Service:    "svc",
				ToStatus:   1,
				ObservedAt: fixedEventTimestamp,
			},
			wantTelegram: "🟢 <b>服务已恢复</b>\n\n" +
				"<b>test</b> / <b>svc</b>\n\n" +
				"时间: " + fixedEventTimeCST,
			wantQQ: "🟢 服务已恢复\n\n" +
				"test / svc\n\n" +
				"时间: " + fixedEventTimeCST,
		},
		{
			name: "无 Type ToStatus 未知值走 default 分支",
			event: &poller.Event{
				Provider:   "test",
				Service:    "svc",
				ToStatus:   99,
				ObservedAt: fixedEventTimestamp,
			},
			wantTelegram: "⚪ <b>状态变更</b>\n\n" +
				"<b>test</b> / <b>svc</b>\n\n" +
				"时间: " + fixedEventTimeCST,
			wantQQ: "⚪ 状态变更\n\n" +
				"test / svc\n\n" +
				"时间: " + fixedEventTimeCST,
		},
		{
			name: "回退 CreatedAt 当 ObservedAt 为 0",
			event: &poller.Event{
				Provider:  "test",
				Service:   "svc",
				Type:      "UP",
				CreatedAt: fixedEventTimestamp,
			},
			wantTelegram: "🟢 <b>服务已恢复</b>\n\n" +
				"<b>test</b> / <b>svc</b>\n\n" +
				"时间: " + fixedEventTimeCST,
			wantQQ: "🟢 服务已恢复\n\n" +
				"test / svc\n\n" +
				"时间: " + fixedEventTimeCST,
		},
	}
}

func TestTelegramChannelFormatMessage(t *testing.T) {
	ch := &telegramChannel{}

	for _, tt := range messageFormatCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := ch.FormatMessage(tt.event)
			if got != tt.wantTelegram {
				t.Fatalf("FormatMessage():\ngot:  %q\nwant: %q", got, tt.wantTelegram)
			}
		})
	}
}

func TestQQChannelFormatMessage(t *testing.T) {
	ch := &qqChannel{}

	for _, tt := range messageFormatCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := ch.FormatMessage(tt.event)
			if got != tt.wantQQ {
				t.Fatalf("FormatMessage():\ngot:  %q\nwant: %q", got, tt.wantQQ)
			}
		})
	}
}
