package monitor

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

var (
	promptRandMu sync.Mutex
	promptRand   = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// GenerateArithmeticPrompt 生成简单加法算术题，用于防止服务商缓存作弊
// 返回提示词和预期答案（格式: RP_ANSWER=<number>）
func GenerateArithmeticPrompt() (prompt, expectedAnswer string) {
	promptRandMu.Lock()
	a := promptRand.Intn(90) + 10 // 10-99
	b := promptRand.Intn(90) + 10
	promptRandMu.Unlock()

	sum := a + b
	expectedAnswer = fmt.Sprintf("RP_ANSWER=%d", sum)
	prompt = fmt.Sprintf("Calculate: %d + %d = ? Reply ONLY: %s", a, b, expectedAnswer)

	return prompt, expectedAnswer
}
