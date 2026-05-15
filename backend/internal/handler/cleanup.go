package handler

import (
	"image2/backend/internal/dto"
	"image2/backend/internal/middleware"
	"image2/backend/internal/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) AdminPreviewDataCleanup(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	var req dto.DataCleanupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}

	resp, err := h.svc.AdminPreviewDataCleanup(c.Request.Context(), adminID, req)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminExecuteDataCleanup(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	var req dto.DataCleanupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}

	resp, err := h.svc.AdminExecuteDataCleanup(c.Request.Context(), adminID, req)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}
