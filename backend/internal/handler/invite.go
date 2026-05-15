package handler

import (
	"image2/backend/internal/middleware"
	"image2/backend/internal/response"

	"github.com/gin-gonic/gin"
)

// GetInviteInfo returns the current user's invite code, rewards, and totals.
func (h *Handler) GetInviteInfo(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	resp, err := h.svc.InviteInfo(c.Request.Context(), userID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response.Success(c, resp)
}
