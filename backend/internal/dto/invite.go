package dto

// InviteInfoResponse is shown in the user invite module.
type InviteInfoResponse struct {
	InviteCode                    string `json:"inviteCode"`
	InvitedCount                  int64  `json:"invitedCount"`
	RewardPointsTotal             int64  `json:"rewardPointsTotal"`
	RewardCreditsTotal            int64  `json:"rewardCreditsTotal"`
	InviteRewardPoints            int64  `json:"inviteRewardPoints"`
	InviteRewardCredits           int64  `json:"inviteRewardCredits"`
	RegistrationInviteBonusPoints int64  `json:"registrationInviteBonusPoints"`
	RegistrationGiftCredits       int64  `json:"registrationGiftCredits"`
}
