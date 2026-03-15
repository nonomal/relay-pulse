package onboarding

import "monitor/internal/apikey"

// ProofIssuer 是 apikey.ProofIssuer 的类型别名，保持向后兼容。
type ProofIssuer = apikey.ProofIssuer

// NewProofIssuer 创建 ProofIssuer。
var NewProofIssuer = apikey.NewProofIssuer
