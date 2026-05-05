package handler

import (
	"errors"

	"image2/backend/internal/dto"
	"image2/backend/internal/middleware"
	"image2/backend/internal/response"
	"image2/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// ChangePassword 允许用户修改自己的登录密码。
func (h *Handler) ChangePassword(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}

	resp, err := h.svc.ChangePassword(c.Request.Context(), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCurrentPasswordIncorrect):
			response.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrPasswordConfirmationMismatch):
			response.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrPasswordUnchanged):
			response.Conflict(c, err.Error())
		default:
			h.writeServiceError(c, err)
		}
		return
	}

	response.Success(c, resp)
}
