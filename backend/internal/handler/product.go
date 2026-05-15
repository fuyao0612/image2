package handler

import (
	"errors"

	"image2/backend/internal/dto"
	"image2/backend/internal/middleware"
	"image2/backend/internal/response"
	"image2/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListProducts(c *gin.Context) {
	resp, err := h.svc.ListProducts(c.Request.Context(), c.Query("keyword"), c.Query("categoryId"))
	if err != nil {
		h.writeProductError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) ListProductCategories(c *gin.Context) {
	resp, err := h.svc.ListProductCategories(c.Request.Context())
	if err != nil {
		h.writeProductError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminListProducts(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	page, pageSize := pagination(c)
	resp, err := h.svc.AdminListProducts(c.Request.Context(), adminID, c.Query("keyword"), c.Query("categoryId"), c.Query("enabled"), page, pageSize)
	if err != nil {
		h.writeProductError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminCreateProduct(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.AdminProductPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.AdminCreateProduct(c.Request.Context(), adminID, req)
	if err != nil {
		h.writeProductError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminUpdateProduct(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.AdminProductPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.AdminUpdateProduct(c.Request.Context(), adminID, c.Param("id"), req)
	if err != nil {
		h.writeProductError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminDeleteProduct(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	if err := h.svc.AdminDeleteProduct(c.Request.Context(), adminID, c.Param("id")); err != nil {
		h.writeProductError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *Handler) AdminListProductCategories(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	page, pageSize := pagination(c)
	resp, err := h.svc.AdminListProductCategories(c.Request.Context(), adminID, page, pageSize)
	if err != nil {
		h.writeProductError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminCreateProductCategory(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.AdminProductCategoryPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.AdminCreateProductCategory(c.Request.Context(), adminID, req)
	if err != nil {
		h.writeProductError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminUpdateProductCategory(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.AdminProductCategoryPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.AdminUpdateProductCategory(c.Request.Context(), adminID, c.Param("id"), req)
	if err != nil {
		h.writeProductError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminDeleteProductCategory(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	if err := h.svc.AdminDeleteProductCategory(c.Request.Context(), adminID, c.Param("id")); err != nil {
		h.writeProductError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *Handler) writeProductError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrProductInvalid):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrProductCategoryInvalid):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrProductCategoryInUse):
		response.Conflict(c, err.Error())
	default:
		h.writeServiceErrorV2(c, err)
	}
}
