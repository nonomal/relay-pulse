package announcements

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetcher_doGraphQL_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求头
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q, want 'Bearer test-token'", r.Header.Get("Authorization"))
		}
		if r.Header.Get("User-Agent") != "RelayPulse/1.0" {
			t.Errorf("User-Agent = %q, want RelayPulse/1.0", r.Header.Get("User-Agent"))
		}

		// 验证请求体
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Variables["owner"] != "test-owner" {
			t.Errorf("variables.owner = %v, want test-owner", body.Variables["owner"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"greeting": "hello",
			},
		})
	}))
	defer server.Close()

	f := &Fetcher{
		client:   server.Client(),
		endpoint: server.URL,
		token:    "test-token",
	}

	var out struct {
		Greeting string `json:"greeting"`
	}
	err := f.doGraphQL(context.Background(), "{ greeting }", map[string]any{"owner": "test-owner"}, &out)
	if err != nil {
		t.Fatalf("doGraphQL() error: %v", err)
	}
	if out.Greeting != "hello" {
		t.Fatalf("Greeting = %q, want 'hello'", out.Greeting)
	}
}

func TestFetcher_doGraphQL_GraphQLError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data":   nil,
			"errors": []map[string]string{{"message": "rate limited"}},
		})
	}))
	defer server.Close()

	f := &Fetcher{client: server.Client(), endpoint: server.URL}

	var out json.RawMessage
	err := f.doGraphQL(context.Background(), "{}", nil, &out)
	if err == nil {
		t.Fatal("expected error for GraphQL error response")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("error = %q, want 'rate limited'", err.Error())
	}
}

func TestFetcher_doGraphQL_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	f := &Fetcher{client: server.Client(), endpoint: server.URL}

	var out json.RawMessage
	err := f.doGraphQL(context.Background(), "{}", nil, &out)
	if err == nil {
		t.Fatal("expected error for HTTP 401")
	}
	if !strings.Contains(err.Error(), "HTTP 状态异常") {
		t.Fatalf("error = %q, want HTTP status error", err.Error())
	}
}

func TestFetcher_doGraphQL_NoToken(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
	}))
	defer server.Close()

	f := &Fetcher{client: server.Client(), endpoint: server.URL, token: ""}

	var out json.RawMessage
	_ = f.doGraphQL(context.Background(), "{}", nil, &out)
	if gotAuth != "" {
		t.Fatalf("Authorization should be empty when no token, got %q", gotAuth)
	}
}

func TestFetcher_FetchCategoryID_CacheHit(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"repository": map[string]any{
					"discussionCategories": map[string]any{
						"nodes": []map[string]string{
							{"id": "CAT_123", "name": "Announcements"},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	f := &Fetcher{client: server.Client(), endpoint: server.URL}

	// 第一次调用
	id1, err := f.FetchCategoryID(context.Background(), "owner", "repo", "Announcements")
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if id1 != "CAT_123" {
		t.Fatalf("id = %q, want CAT_123", id1)
	}

	// 第二次调用应命中缓存
	id2, err := f.FetchCategoryID(context.Background(), "owner", "repo", "announcements") // 大小写不同
	if err != nil {
		t.Fatalf("cached call error: %v", err)
	}
	if id2 != "CAT_123" {
		t.Fatalf("cached id = %q, want CAT_123", id2)
	}

	if callCount != 1 {
		t.Fatalf("server called %d times, want 1 (cache miss + cache hit)", callCount)
	}
}

func TestFetcher_FetchCategoryID_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"repository": map[string]any{
					"discussionCategories": map[string]any{
						"nodes": []map[string]string{
							{"id": "CAT_OTHER", "name": "General"},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	f := &Fetcher{client: server.Client(), endpoint: server.URL}

	_, err := f.FetchCategoryID(context.Background(), "owner", "repo", "Announcements")
	if err == nil {
		t.Fatal("expected error for missing category")
	}
	if !strings.Contains(err.Error(), "未找到") {
		t.Fatalf("error = %q, want '未找到'", err.Error())
	}
}

func TestFetcher_FetchCategoryID_EmptyParams(t *testing.T) {
	f := &Fetcher{client: http.DefaultClient, endpoint: "http://unused"}

	_, err := f.FetchCategoryID(context.Background(), "", "repo", "cat")
	if err == nil {
		t.Fatal("empty owner should return error")
	}
	_, err = f.FetchCategoryID(context.Background(), "owner", "", "cat")
	if err == nil {
		t.Fatal("empty repo should return error")
	}
	_, err = f.FetchCategoryID(context.Background(), "owner", "repo", "")
	if err == nil {
		t.Fatal("empty category should return error")
	}
}

func TestFetcher_FetchDiscussionsByCategoryID_ParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"repository": map[string]any{
					"discussions": map[string]any{
						"nodes": []map[string]any{
							{
								"id": "D_1", "number": 42, "title": "公告标题",
								"url":       "https://github.com/o/r/discussions/42",
								"createdAt": "2024-01-15T10:30:00Z",
								"author":    map[string]string{"login": "alice"},
							},
							{
								"id": "D_2", "number": 41, "title": "无作者公告",
								"url":       "https://github.com/o/r/discussions/41",
								"createdAt": "2024-01-14T08:00:00Z",
								"author":    nil,
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	f := &Fetcher{client: server.Client(), endpoint: server.URL}

	discussions, err := f.FetchDiscussionsByCategoryID(context.Background(), "o", "r", "CAT_1", 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(discussions) != 2 {
		t.Fatalf("len = %d, want 2", len(discussions))
	}

	// 第一条
	if discussions[0].Number != 42 || discussions[0].Title != "公告标题" {
		t.Fatalf("discussion[0] = %+v", discussions[0])
	}
	if discussions[0].AuthorLogin != "alice" {
		t.Fatalf("AuthorLogin = %q, want alice", discussions[0].AuthorLogin)
	}
	expectedTime, _ := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
	if !discussions[0].CreatedAt.Equal(expectedTime) {
		t.Fatalf("CreatedAt = %v, want %v", discussions[0].CreatedAt, expectedTime)
	}

	// 第二条（无 author）
	if discussions[1].AuthorLogin != "" {
		t.Fatalf("nil author should give empty login, got %q", discussions[1].AuthorLogin)
	}
}

func TestFetcher_FetchDiscussionsByCategoryID_DefaultFirst(t *testing.T) {
	var gotFirst float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Variables map[string]any `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		gotFirst = body.Variables["first"].(float64)

		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"repository": map[string]any{
					"discussions": map[string]any{"nodes": []any{}},
				},
			},
		})
	}))
	defer server.Close()

	f := &Fetcher{client: server.Client(), endpoint: server.URL}

	// first=0 应默认为 20
	_, _ = f.FetchDiscussionsByCategoryID(context.Background(), "o", "r", "CAT", 0)
	if gotFirst != 20 {
		t.Fatalf("first = %v, want 20 (default)", gotFirst)
	}
}
