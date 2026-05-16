package dto

import (
	"testing"

	"github.com/gin-gonic/gin/binding"
)

func TestUpdateBillingSettingsRequestAllowsZeroRewardFields(t *testing.T) {
	payload := []byte(`{
		"imageGeneratePointCost": 20,
		"checkinPoints": 0,
		"checkinCredits": 0,
		"emailNotifyPointCost": 0,
		"inviteRewardPoints": 0,
		"inviteRewardCredits": 0,
		"registrationInviteBonusPoints": 0,
		"registrationGiftCredits": 0
	}`)

	var req UpdateBillingSettingsRequest
	if err := binding.JSON.BindBody(payload, &req); err != nil {
		t.Fatalf("expected zero reward fields to pass validation, got %v", err)
	}
}

func TestUpdateBillingSettingsRequestRequiresRewardFields(t *testing.T) {
	payload := []byte(`{
		"imageGeneratePointCost": 20,
		"checkinPoints": 0,
		"checkinCredits": 0,
		"emailNotifyPointCost": 0,
		"inviteRewardCredits": 0,
		"registrationInviteBonusPoints": 0,
		"registrationGiftCredits": 0
	}`)

	var req UpdateBillingSettingsRequest
	if err := binding.JSON.BindBody(payload, &req); err == nil {
		t.Fatal("expected missing inviteRewardPoints to fail validation")
	}
}
