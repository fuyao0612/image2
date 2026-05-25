package dto

// BillingSettingsResponse represents editable platform billing/reward settings.
type BillingSettingsResponse struct {
	ImageGeneratePointCost        int64 `json:"imageGeneratePointCost"`
	ImageGenerationEnabled        bool  `json:"imageGenerationEnabled"`
	ImageFailureNotifyEnabled     bool  `json:"imageFailureNotifyEnabled"`
	EmailNotifyEnabled            bool  `json:"emailNotifyEnabled"`
	EmailNotifyPointCost          int64 `json:"emailNotifyPointCost"`
	CheckinPoints                 int64 `json:"checkinPoints"`
	CheckinCredits                int64 `json:"checkinCredits"`
	InviteRewardPoints            int64 `json:"inviteRewardPoints"`
	InviteRewardCredits           int64 `json:"inviteRewardCredits"`
	RegistrationInviteBonusPoints int64 `json:"registrationInviteBonusPoints"`
	RegistrationGiftCredits       int64 `json:"registrationGiftCredits"`
}

// UpdateBillingSettingsRequest updates editable billing/reward settings.
type UpdateBillingSettingsRequest struct {
	ImageGeneratePointCost        int64  `json:"imageGeneratePointCost" binding:"required,min=1"`
	ImageGenerationEnabled        *bool  `json:"imageGenerationEnabled"`
	ImageFailureNotifyEnabled     *bool  `json:"imageFailureNotifyEnabled"`
	EmailNotifyEnabled            *bool  `json:"emailNotifyEnabled"`
	EmailNotifyPointCost          *int64 `json:"emailNotifyPointCost" binding:"required,min=0"`
	CheckinPoints                 *int64 `json:"checkinPoints" binding:"required,min=0"`
	CheckinCredits                *int64 `json:"checkinCredits" binding:"required,min=0"`
	InviteRewardPoints            *int64 `json:"inviteRewardPoints" binding:"required,min=0"`
	InviteRewardCredits           *int64 `json:"inviteRewardCredits" binding:"required,min=0"`
	RegistrationInviteBonusPoints *int64 `json:"registrationInviteBonusPoints" binding:"required,min=0"`
	RegistrationGiftCredits       *int64 `json:"registrationGiftCredits" binding:"required,min=0"`
}
