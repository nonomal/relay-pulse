package selftest

import "errors"

// ErrorCode 自助测试错误码（前端可稳定依赖，不依赖 error 字符串）
type ErrorCode string

const (
	// ErrCodeBadRequest 请求参数不合法
	ErrCodeBadRequest ErrorCode = "bad_request"
	// ErrCodeRateLimited 触发限流
	ErrCodeRateLimited ErrorCode = "rate_limited"
	// ErrCodeFeatureDisabled 功能未启用
	ErrCodeFeatureDisabled ErrorCode = "feature_disabled"
	// ErrCodeInvalidURL URL 不安全或不合法
	ErrCodeInvalidURL ErrorCode = "invalid_url"
	// ErrCodeUnknownTestType 测试类型不支持
	ErrCodeUnknownTestType ErrorCode = "unknown_test_type"
	// ErrCodeQueueFull 队列已满
	ErrCodeQueueFull ErrorCode = "queue_full"
	// ErrCodeJobNotFound 任务不存在或已过期
	ErrCodeJobNotFound ErrorCode = "job_not_found"
)

// Error 自助测试领域错误（对外 Message + 稳定 Code；Err 用于内部诊断）
type Error struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Err }

// CodeOf 提取自助测试错误码；若不是 selftest.Error 则返回空串
func CodeOf(err error) ErrorCode {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}
