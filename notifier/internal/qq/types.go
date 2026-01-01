package qq

import "encoding/json"

// OneBotEvent OneBot v11 上报事件（消息、元事件等）
type OneBotEvent struct {
	Time   int64 `json:"time"`
	SelfID int64 `json:"self_id"`

	PostType string `json:"post_type"` // message / meta_event / notice / request

	// message 相关字段
	MessageType string          `json:"message_type,omitempty"` // private / group
	SubType     string          `json:"sub_type,omitempty"`
	MessageID   int64           `json:"message_id,omitempty"`
	GroupID     int64           `json:"group_id,omitempty"`
	UserID      int64           `json:"user_id,omitempty"`
	Message     json.RawMessage `json:"message,omitempty"`     // string 或 segment[]
	RawMessage  string          `json:"raw_message,omitempty"` // 原始消息文本
	Font        int             `json:"font,omitempty"`
	Sender      *Sender         `json:"sender,omitempty"`

	// meta_event / notice / request（预留字段）
	MetaEventType string `json:"meta_event_type,omitempty"`
	NoticeType    string `json:"notice_type,omitempty"`
	RequestType   string `json:"request_type,omitempty"`
}

// Sender 消息发送者信息
type Sender struct {
	UserID   int64  `json:"user_id,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Card     string `json:"card,omitempty"` // 群名片
	Role     string `json:"role,omitempty"` // owner / admin / member（群消息时）
}

// MessageSegment OneBot v11 消息段
type MessageSegment struct {
	Type string `json:"type"`
	Data struct {
		Text string `json:"text,omitempty"`
	} `json:"data,omitempty"`
}

// GroupMember 群成员信息（用于权限判断）
type GroupMember struct {
	GroupID  int64  `json:"group_id"`
	UserID   int64  `json:"user_id"`
	Role     string `json:"role"` // owner / admin / member
	Nickname string `json:"nickname,omitempty"`
	Card     string `json:"card,omitempty"`
	Title    string `json:"title,omitempty"`
	Level    string `json:"level,omitempty"`
}

// APIResponse OneBot HTTP API 响应
type APIResponse struct {
	Status  string          `json:"status"`  // ok / failed
	RetCode int             `json:"retcode"` // 0 表示成功
	Data    json.RawMessage `json:"data,omitempty"`
	Msg     string          `json:"msg,omitempty"`
	Wording string          `json:"wording,omitempty"`
	Echo    json.RawMessage `json:"echo,omitempty"`
}

// sendMsgResult 发送消息返回结果
type sendMsgResult struct {
	MessageID int64 `json:"message_id"`
}
