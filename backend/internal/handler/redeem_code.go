package handler

import (
	"image2/backend/internal/dto"
	"image2/backend/internal/middleware"
	"image2/backend/internal/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) RedeemCode(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	var req dto.RedeemCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}

	resp, err := h.svc.RedeemCode(c.Request.Context(), userID, req)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminCreateRedeemCodes(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	var req dto.AdminCreateRedeemCodesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}

	resp, err := h.svc.AdminCreateRedeemCodes(c.Request.Context(), adminID, req)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminListRedeemCodes(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	page, pageSize := pagination(c)
	resp, err := h.svc.AdminListRedeemCodes(c.Request.Context(), adminID, c.Query("status"), c.Query("keyword"), page, pageSize)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminDeleteRedeemCode(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	if err := h.svc.AdminDeleteRedeemCode(c.Request.Context(), adminID, c.Param("id")); err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
