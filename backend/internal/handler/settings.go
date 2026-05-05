package handler

import (
	"image2/backend/internal/dto"
	"image2/backend/internal/middleware"
	"image2/backend/internal/response"

	"github.com/gin-gonic/gin"
)

// AdminGetBillingSettings 返回后台可配置的账本规则。
func (h *Handler) AdminGetBillingSettings(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	if err := h.svc.RequireAdmin(c.Request.Context(), adminID); err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	resp, err := h.svc.BillingSettings(c.Request.Context())
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminUpdateBillingSettings 更新后台可配置的账本规则。
func (h *Handler) AdminUpdateBillingSettings(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.UpdateBillingSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.UpdateBillingSettings(c.Request.Context(), adminID, req)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}
