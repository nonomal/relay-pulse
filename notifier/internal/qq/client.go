package qq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client OneBot HTTP API 客户端（OneBot v11 / NapCatQQ）
type Client struct {
	baseURL     string
	accessToken string
	httpClient  *http.Client
}

// NewClient 创建 OneBot HTTP API 客户端
func NewClient(baseURL, accessToken string) *Client {
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// SendGroupMessage 发送群消息（纯文本）
func (c *Client) SendGroupMessage(ctx context.Context, groupID int64, text string) (int64, error) {
	params := map[string]interface{}{
		"group_id":    groupID,
		"message":     text,
		"auto_escape": true, // 自动转义，避免 CQ 码注入
	}

	resp, err := c.doRequest(ctx, "send_group_msg", params)
	if err != nil {
		return 0, err
	}

	var result sendMsgResult
	if len(resp.Data) > 0 {
		if err := json.Unmarshal(resp.Data, &result); err != nil {
			return 0, fmt.Errorf("解析发送结果失败: %w", err)
		}
	}
	return result.MessageID, nil
}

// SendPrivateMessage 发送私聊消息（纯文本）
func (c *Client) SendPrivateMessage(ctx context.Context, userID int64, text string) (int64, error) {
	params := map[string]interface{}{
		"user_id":     userID,
		"message":     text,
		"auto_escape": true,
	}

	resp, err := c.doRequest(ctx, "send_private_msg", params)
	if err != nil {
		return 0, err
	}

	var result sendMsgResult
	if len(resp.Data) > 0 {
		if err := json.Unmarshal(resp.Data, &result); err != nil {
			return 0, fmt.Errorf("解析发送结果失败: %w", err)
		}
	}
	return result.MessageID, nil
}

// SendGroupImageMessage 发送群图片消息（使用消息段格式）
func (c *Client) SendGroupImageMessage(ctx context.Context, groupID int64, base64Data string) (int64, error) {
	// 使用消息段数组格式，NapCatQQ 更好支持
	message := []map[string]interface{}{
		{
			"type": "image",
			"data": map[string]string{
				"file": "base64://" + base64Data,
			},
		},
	}

	params := map[string]interface{}{
		"group_id": groupID,
		"message":  message,
	}

	resp, err := c.doRequest(ctx, "send_group_msg", params)
	if err != nil {
		return 0, err
	}

	var result sendMsgResult
	if len(resp.Data) > 0 {
		if err := json.Unmarshal(resp.Data, &result); err != nil {
			return 0, fmt.Errorf("解析发送结果失败: %w", err)
		}
	}
	return result.MessageID, nil
}

// SendPrivateImageMessage 发送私聊图片消息（使用消息段格式）
func (c *Client) SendPrivateImageMessage(ctx context.Context, userID int64, base64Data string) (int64, error) {
	message := []map[string]interface{}{
		{
			"type": "image",
			"data": map[string]string{
				"file": "base64://" + base64Data,
			},
		},
	}

	params := map[string]interface{}{
		"user_id": userID,
		"message": message,
	}

	resp, err := c.doRequest(ctx, "send_private_msg", params)
	if err != nil {
		return 0, err
	}

	var result sendMsgResult
	if len(resp.Data) > 0 {
		if err := json.Unmarshal(resp.Data, &result); err != nil {
			return 0, fmt.Errorf("解析发送结果失败: %w", err)
		}
	}
	return result.MessageID, nil
}

// GetGroupMemberInfo 获取群成员信息（用于权限二次确认）
func (c *Client) GetGroupMemberInfo(ctx context.Context, groupID, userID int64) (*GroupMember, error) {
	params := map[string]interface{}{
		"group_id": groupID,
		"user_id":  userID,
		"no_cache": true, // 不使用缓存，确保权限实时
	}

	resp, err := c.doRequest(ctx, "get_group_member_info", params)
	if err != nil {
		return nil, err
	}

	var member GroupMember
	if err := json.Unmarshal(resp.Data, &member); err != nil {
		return nil, fmt.Errorf("解析群成员信息失败: %w", err)
	}
	return &member, nil
}

// GetGroupInfo 获取群信息
func (c *Client) GetGroupInfo(ctx context.Context, groupID int64) (*GroupInfo, error) {
	params := map[string]interface{}{
		"group_id": groupID,
	}

	resp, err := c.doRequest(ctx, "get_group_info", params)
	if err != nil {
		return nil, err
	}

	var info GroupInfo
	if err := json.Unmarshal(resp.Data, &info); err != nil {
		return nil, fmt.Errorf("解析群信息失败: %w", err)
	}
	return &info, nil
}

// doRequest 执行 API 请求
func (c *Client) doRequest(ctx context.Context, action string, params map[string]interface{}) (*APIResponse, error) {
	url := c.baseURL + "/" + strings.TrimLeft(action, "/")

	var body io.Reader
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("序列化参数失败: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP 错误 [%d]: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if apiResp.Status != "ok" || apiResp.RetCode != 0 {
		msg := apiResp.Msg
		if msg == "" {
			msg = apiResp.Wording
		}
		if msg == "" {
			msg = "unknown error"
		}
		return &apiResp, fmt.Errorf("OneBot API 错误 [%d]: %s", apiResp.RetCode, msg)
	}

	return &apiResp, nil
}
