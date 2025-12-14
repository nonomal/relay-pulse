package selftest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math"
	"time"
)

// SignatureValidator validates HMAC-SHA256 signatures for request authentication
// This prevents non-page submissions by requiring a secret key
type SignatureValidator struct {
	secretKey       string        // Server secret key (configured)
	timestampWindow time.Duration // Timestamp validity window (default 5 minutes)
}

// NewSignatureValidator creates a new signature validator
func NewSignatureValidator(secretKey string) *SignatureValidator {
	return &SignatureValidator{
		secretKey:       secretKey,
		timestampWindow: 5 * time.Minute, // Default 5-minute window
	}
}

// SetTimestampWindow sets the timestamp validity window
func (v *SignatureValidator) SetTimestampWindow(window time.Duration) {
	v.timestampWindow = window
}

// GenerateSignature generates an HMAC-SHA256 signature for the given parameters
// Message format: "timestamp:test_type:api_url"
func (v *SignatureValidator) GenerateSignature(timestamp int64, testType, apiURL string) string {
	// Construct message
	message := fmt.Sprintf("%d:%s:%s", timestamp, testType, apiURL)

	// HMAC-SHA256
	h := hmac.New(sha256.New, []byte(v.secretKey))
	h.Write([]byte(message))

	// Base64 encoding
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ValidateSignature validates a signature against the expected signature
// Returns nil if valid, error otherwise
func (v *SignatureValidator) ValidateSignature(timestamp int64, testType, apiURL, signature string) error {
	// 1. Timestamp validation (prevent replay attacks)
	now := time.Now().Unix()
	timeDiff := int64(math.Abs(float64(now - timestamp)))

	if timeDiff > int64(v.timestampWindow.Seconds()) {
		return fmt.Errorf("timestamp expired (diff: %d seconds, max: %d seconds)",
			timeDiff, int64(v.timestampWindow.Seconds()))
	}

	// 2. Signature validation
	expected := v.GenerateSignature(timestamp, testType, apiURL)

	// Use constant-time comparison to prevent timing attacks
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}
