package provider

import (
	"context"
	"fmt"
	"strings"
)

// Submit 将批次提交到第三方服务，仅返回提交阶段的结果。
func (p *MentaloutProvider) Submit(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	if !p.Configured() {
		return GenerateResult{}, ErrNotConfigured
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return GenerateResult{}, fmt.Errorf("prompt cannot be empty")
	}

	return p.submitBatch(ctx, req)
}

// Fetch 根据批次 ID 拉取当前状态。
func (p *MentaloutProvider) Fetch(ctx context.Context, batchID string) (GenerateResult, error) {
	if !p.Configured() {
		return GenerateResult{}, ErrNotConfigured
	}
	if strings.TrimSpace(batchID) == "" {
		return GenerateResult{}, fmt.Errorf("batch id cannot be empty")
	}

	return p.fetchBatch(ctx, batchID)
}
