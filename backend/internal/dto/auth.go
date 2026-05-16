package dto

import "time"

// SendEmailCodeRequest 是发送邮箱验证码的请求体。
type SendEmailCodeRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Scene       string `json:"scene" binding:"required,oneof=register reset_password"`
	CaptchaID   string `json:"captchaId,omitempty"`
	CaptchaCode string `json:"captchaCode,omitempty"`
}

// SendEmailCodeResponse 是发送验证码的响应体。
type SendEmailCodeResponse struct {
	Sent      bool   `json:"sent"`
	ExpiresIn int    `json:"expiresIn"`
	DebugCode string `json:"debugCode,omitempty"`
}

type CaptchaResponse struct {
	CaptchaID string `json:"captchaId"`
	Image     string `json:"image"`
	ExpiresIn int    `json:"expiresIn"`
}

// RegisterRequest 是注册请求体。
type RegisterRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Code       string `json:"code" binding:"required"`
	Username   string `json:"username" binding:"required,min=2,max=32"`
	Password   string `json:"password" binding:"required,min=8,max=72"`
	InviteCode string `json:"inviteCode,omitempty" binding:"omitempty,max=16"`
}

// LoginRequest 是登录请求体。
type LoginRequest struct {
	Account    string `json:"account" binding:"required"`
	Password   string `json:"password" binding:"required"`
	RememberMe bool   `json:"rememberMe,omitempty"`
}

type CheckResetPasswordEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type CheckRegisterAvailabilityRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,min=2,max=32"`
}

type CheckRegisterAvailabilityResponse struct {
	EmailAvailable    bool   `json:"emailAvailable"`
	UsernameAvailable bool   `json:"usernameAvailable"`
	Available         bool   `json:"available"`
	Message           string `json:"message,omitempty"`
}

type CheckResetPasswordEmailResponse struct {
	Available    bool       `json:"available"`
	Message      string     `json:"message,omitempty"`
	NextResetAt  *time.Time `json:"nextResetAt,omitempty"`
	ResetAfterAt *time.Time `json:"resetAfterAt,omitempty"`
}

type ResetPasswordRequest struct {
	Email           string `json:"email" binding:"required,email"`
	Code            string `json:"code" binding:"required"`
	Password        string `json:"password" binding:"required,min=8,max=72"`
	ConfirmPassword string `json:"confirmPassword" binding:"required,min=8,max=72"`
}

// UserInfo 是统一的用户返回结构。
type UserInfo struct {
	ID            string     `json:"id"`
	Username      string     `json:"username"`
	Email         string     `json:"email"`
	Role          string     `json:"role"`
	PointsBalance int64      `json:"pointsBalance"`
	FreeCredits   int64      `json:"freeCredits"`
	PaidCredits   int64      `json:"paidCredits"`
	TotalCredits  int64      `json:"totalCredits"`
	CheckinStreak int        `json:"checkinStreak"`
	LastCheckinAt *time.Time `json:"lastCheckinAt,omitempty"`
	LastLoginAt   *time.Time `json:"lastLoginAt,omitempty"`
	InviteCode    string     `json:"inviteCode,omitempty"`
}

// AuthResponse 是登录或注册成功后的统一返回体。
type AuthResponse struct {
	AccessToken string   `json:"accessToken"`
	TokenType   string   `json:"tokenType"`
	ExpiresIn   int64    `json:"expiresIn"`
	User        UserInfo `json:"user"`
}

// ChangePasswordRequest 是用户修改密码的请求体。
type ChangePasswordRequest struct {
	CurrentPassword    string `json:"currentPassword" binding:"required,min=1,max=72"`
	NewPassword        string `json:"newPassword" binding:"required,min=8,max=72"`
	ConfirmNewPassword string `json:"confirmNewPassword" binding:"required,min=8,max=72"`
}

// ChangePasswordResponse 是用户修改密码后的返回体。
type ChangePasswordResponse struct {
	ChangedAt time.Time `json:"changedAt"`
}
