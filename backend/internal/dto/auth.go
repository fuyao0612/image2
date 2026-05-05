package dto

import "time"

// SendEmailCodeRequest 是发送邮箱验证码的请求体。
type SendEmailCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
	Scene string `json:"scene" binding:"required,oneof=register reset_password"`
}

// SendEmailCodeResponse 是发送验证码的响应体。
type SendEmailCodeResponse struct {
	Sent      bool   `json:"sent"`
	ExpiresIn int    `json:"expiresIn"`
	DebugCode string `json:"debugCode,omitempty"`
}

// RegisterRequest 是注册请求体。
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Code     string `json:"code" binding:"required"`
	Username string `json:"username" binding:"required,min=2,max=32"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

// LoginRequest 是登录请求体。
type LoginRequest struct {
	Account  string `json:"account" binding:"required"`
	Password string `json:"password" binding:"required"`
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
