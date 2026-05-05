package service

import (
	"context"
	"encoding/json"
	"strings"

	"image2/backend/internal/model"
)

type contentAuditDecision struct {
	Status          model.ContentAuditStatus
	Action          string
	Reason          string
	MatchedKeywords []string
}

func (s *Service) moderatePrompt(prompt string) contentAuditDecision {
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	if normalized == "" {
		return contentAuditDecision{
			Status: model.ContentAuditStatusRejected,
			Action: "reject",
			Reason: "prompt 不能为空",
		}
	}

	matched := make([]string, 0)
	for _, keyword := range s.cfg.PromptBlockKeywords {
		trimmed := strings.TrimSpace(keyword)
		if trimmed == "" {
			continue
		}
		if strings.Contains(normalized, strings.ToLower(trimmed)) {
			matched = append(matched, trimmed)
		}
	}
	if len(matched) > 0 {
		return contentAuditDecision{
			Status:          model.ContentAuditStatusRejected,
			Action:          "reject",
			Reason:          "命中平台禁止生成的内容规则",
			MatchedKeywords: matched,
		}
	}

	return contentAuditDecision{
		Status: model.ContentAuditStatusPass,
		Action: "allow",
		Reason: "规则审核通过",
	}
}

func (s *Service) recordContentAudit(ctx context.Context, userID uint64, bizType string, bizID string, contentType string, content string, decision contentAuditDecision) error {
	if ctx == nil {
		ctx = context.Background()
	}
	rawKeywords, _ := json.Marshal(decision.MatchedKeywords)
	log := model.ContentAuditLog{
		UserID:          userID,
		BizType:         strings.TrimSpace(bizType),
		BizID:           strings.TrimSpace(bizID),
		ContentType:     strings.TrimSpace(contentType),
		Content:         strings.TrimSpace(content),
		Status:          decision.Status,
		MatchedKeywords: string(rawKeywords),
		Reason:          decision.Reason,
		Action:          decision.Action,
	}
	return s.db.WithContext(ctx).Create(&log).Error
}
