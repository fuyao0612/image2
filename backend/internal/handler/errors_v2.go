package handler

import (
	"errors"

	"image2/backend/internal/response"
	"image2/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// writeServiceErrorV2 面向新订单流和后台审计的错误映射。
func (h *Handler) writeServiceErrorV2(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidVerificationCode):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrAccountExists):
		response.Conflict(c, err.Error())
	case errors.Is(err, service.ErrInvalidCredentials):
		response.Unauthorized(c, err.Error())
	case errors.Is(err, service.ErrUserDisabled):
		response.Forbidden(c, err.Error())
	case errors.Is(err, service.ErrAlreadyCheckedIn):
		response.Conflict(c, err.Error())
	case errors.Is(err, service.ErrInsufficientPoints):
		response.Conflict(c, err.Error())
	case errors.Is(err, service.ErrInsufficientCredits):
		response.Conflict(c, err.Error())
	case errors.Is(err, service.ErrUserNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrTaskNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrTaskStillGenerating):
		response.Conflict(c, err.Error())
	case errors.Is(err, service.ErrOrderNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrOrderInvalidState):
		response.Conflict(c, err.Error())
	case errors.Is(err, service.ErrOrderAmountMismatch):
		response.Conflict(c, err.Error())
	case errors.Is(err, service.ErrPackageInvalid):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrPackageProtected):
		response.Conflict(c, err.Error())
	case errors.Is(err, service.ErrSettingsInvalid):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrPromptRequired):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrProviderNotConfigured):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrEmailSenderNotConfigured):
		response.InternalError(c, err.Error())
	case errors.Is(err, service.ErrForbidden):
		response.Forbidden(c, err.Error())
	case errors.Is(err, service.ErrContentRejected):
		response.BadRequest(c, err.Error())
	default:
		response.InternalError(c, "服务端异常")
	}
}
