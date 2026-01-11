// Package announcements 提供 GitHub Discussions 公告通知功能
package announcements

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"monitor/internal/config"
)

const githubGraphQLEndpoint = "https://api.github.com/graphql"

// Discussion GitHub Discussion 数据（仅包含公告需要的字段）
type Discussion struct {
	ID          string
	Number      int
	Title       string
	URL         string
	CreatedAt   time.Time
	AuthorLogin string
}

// Fetcher GitHub GraphQL 拉取器
type Fetcher struct {
	client   *http.Client
	endpoint string
	token    string

	// 分类 ID 缓存（避免每次都查询）
	mu                 sync.Mutex
	cachedCategoryID   string
	cachedOwner        string
	cachedRepo         string
	cachedCategoryName string
	cachedCategoryAt   time.Time
}

// NewFetcher 创建 GitHub GraphQL 拉取器（支持代理）
func NewFetcher(cfg config.GitHubConfig) (*Fetcher, error) {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}

	// 如果配置了代理，使用配置的代理
	if strings.TrimSpace(cfg.Proxy) != "" {
		proxyURL, err := url.Parse(strings.TrimSpace(cfg.Proxy))
		if err != nil {
			return nil, fmt.Errorf("解析 github.proxy 失败: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := &http.Client{
		Timeout:   cfg.TimeoutDuration,
		Transport: transport,
	}

	return &Fetcher{
		client:   client,
		endpoint: githubGraphQLEndpoint,
		token:    strings.TrimSpace(cfg.Token),
	}, nil
}

// FetchCategoryID 获取指定 Discussions 分类的 ID（带 24h 缓存）
func (f *Fetcher) FetchCategoryID(ctx context.Context, owner, repo, categoryName string) (string, error) {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	categoryName = strings.TrimSpace(categoryName)

	if owner == "" || repo == "" || categoryName == "" {
		return "", fmt.Errorf("获取分类 ID 失败: owner/repo/category_name 不能为空")
	}

	// 检查缓存
	f.mu.Lock()
	if f.cachedCategoryID != "" &&
		f.cachedOwner == owner &&
		f.cachedRepo == repo &&
		strings.EqualFold(f.cachedCategoryName, categoryName) &&
		time.Since(f.cachedCategoryAt) < 24*time.Hour {
		id := f.cachedCategoryID
		f.mu.Unlock()
		return id, nil
	}
	f.mu.Unlock()

	// 查询分类列表
	const query = `
query($owner: String!, $name: String!) {
  repository(owner: $owner, name: $name) {
    discussionCategories(first: 100) {
      nodes { id name }
    }
  }
}`

	var resp struct {
		Repository struct {
			DiscussionCategories struct {
				Nodes []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"nodes"`
			} `json:"discussionCategories"`
		} `json:"repository"`
	}

	variables := map[string]any{
		"owner": owner,
		"name":  repo,
	}

	if err := f.doGraphQL(ctx, query, variables, &resp); err != nil {
		return "", err
	}

	// 查找匹配的分类
	for _, node := range resp.Repository.DiscussionCategories.Nodes {
		if strings.EqualFold(strings.TrimSpace(node.Name), categoryName) {
			// 更新缓存
			f.mu.Lock()
			f.cachedCategoryID = node.ID
			f.cachedOwner = owner
			f.cachedRepo = repo
			f.cachedCategoryName = categoryName
			f.cachedCategoryAt = time.Now()
			f.mu.Unlock()
			return node.ID, nil
		}
	}

	return "", fmt.Errorf("未找到 Discussions 分类: %s", categoryName)
}

// FetchDiscussionsByCategoryID 获取指定分类下的讨论列表（按 createdAt 倒序）
func (f *Fetcher) FetchDiscussionsByCategoryID(ctx context.Context, owner, repo, categoryID string, first int) ([]Discussion, error) {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	categoryID = strings.TrimSpace(categoryID)

	if owner == "" || repo == "" || categoryID == "" {
		return nil, fmt.Errorf("获取 discussions 失败: owner/repo/category_id 不能为空")
	}

	if first <= 0 {
		first = 20
	}

	const query = `
query($owner: String!, $name: String!, $categoryId: ID!, $first: Int!) {
  repository(owner: $owner, name: $name) {
    discussions(first: $first, categoryId: $categoryId, orderBy: {field: CREATED_AT, direction: DESC}) {
      nodes {
        id
        number
        title
        url
        createdAt
        author { login }
      }
    }
  }
}`

	var resp struct {
		Repository struct {
			Discussions struct {
				Nodes []struct {
					ID        string `json:"id"`
					Number    int    `json:"number"`
					Title     string `json:"title"`
					URL       string `json:"url"`
					CreatedAt string `json:"createdAt"`
					Author    *struct {
						Login string `json:"login"`
					} `json:"author"`
				} `json:"nodes"`
			} `json:"discussions"`
		} `json:"repository"`
	}

	variables := map[string]any{
		"owner":      owner,
		"name":       repo,
		"categoryId": categoryID,
		"first":      first,
	}

	if err := f.doGraphQL(ctx, query, variables, &resp); err != nil {
		return nil, err
	}

	// 转换为 Discussion 结构
	discussions := make([]Discussion, 0, len(resp.Repository.Discussions.Nodes))
	for _, node := range resp.Repository.Discussions.Nodes {
		createdAt, err := time.Parse(time.RFC3339, node.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("解析 discussion.createdAt 失败: %w", err)
		}

		authorLogin := ""
		if node.Author != nil {
			authorLogin = strings.TrimSpace(node.Author.Login)
		}

		discussions = append(discussions, Discussion{
			ID:          node.ID,
			Number:      node.Number,
			Title:       node.Title,
			URL:         node.URL,
			CreatedAt:   createdAt,
			AuthorLogin: authorLogin,
		})
	}

	return discussions, nil
}

// doGraphQL 执行 GraphQL 请求
func (f *Fetcher) doGraphQL(ctx context.Context, query string, variables map[string]any, out any) error {
	type requestBody struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables,omitempty"`
	}

	body, err := json.Marshal(requestBody{
		Query:     query,
		Variables: variables,
	})
	if err != nil {
		return fmt.Errorf("构造 GraphQL 请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建 GraphQL 请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "RelayPulse/1.0")

	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求 GitHub GraphQL 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub GraphQL HTTP 状态异常: %s", resp.Status)
	}

	var rawResp struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawResp); err != nil {
		return fmt.Errorf("解析 GitHub GraphQL 响应失败: %w", err)
	}

	if len(rawResp.Errors) > 0 {
		msg := strings.TrimSpace(rawResp.Errors[0].Message)
		if msg == "" {
			msg = "unknown error"
		}
		return fmt.Errorf("GitHub GraphQL 返回错误: %s", msg)
	}

	if len(rawResp.Data) == 0 {
		return fmt.Errorf("GitHub GraphQL 返回空 data")
	}

	if err := json.Unmarshal(rawResp.Data, out); err != nil {
		return fmt.Errorf("解析 GitHub GraphQL data 失败: %w", err)
	}

	return nil
}
