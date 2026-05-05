package handler

import (
	"image2/backend/internal/dto"
	"image2/backend/internal/middleware"
	"image2/backend/internal/response"

	"github.com/gin-gonic/gin"
)

// CreateRechargeOrder 创建待支付充值订单。
func (h *Handler) CreateRechargeOrder(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	var req dto.CreateRechargeOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}

	resp, err := h.svc.CreateRechargeOrder(c.Request.Context(), userID, req)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}

	response.Success(c, resp)
}

// ConfirmRechargeOrder 确认指定充值订单已支付。
func (h *Handler) ConfirmRechargeOrder(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	var req dto.ConfirmRechargeOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}

	resp, err := h.svc.ConfirmRechargeOrder(c.Request.Context(), userID, c.Param("orderNo"), req)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}

	response.Success(c, resp)
}

// RechargePaymentCallback 是支付渠道回调入口。
func (h *Handler) RechargePaymentCallback(c *gin.Context) {
	var req dto.RechargePaymentCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}

	resp, err := h.svc.RechargePaymentCallback(c.Request.Context(), req)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}

	response.Success(c, resp)
}

// ListRechargeOrders 返回当前用户的充值订单列表。
func (h *Handler) ListRechargeOrders(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	page, pageSize := pagination(c)
	resp, err := h.svc.ListRechargeOrders(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}

	response.Success(c, resp)
}

// GetRechargeOrder 返回当前用户指定充值订单详情。
func (h *Handler) GetRechargeOrder(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	resp, err := h.svc.GetRechargeOrder(c.Request.Context(), userID, c.Param("orderNo"))
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}

	response.Success(c, resp)
}
