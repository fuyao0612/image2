package handler

import (
	"image2/backend/internal/dto"
	"image2/backend/internal/middleware"
	"image2/backend/internal/response"

	"github.com/gin-gonic/gin"
)

// AdminListUserActivity 返回指定用户的业务日志。
func (h *Handler) AdminListUserActivity(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	page, pageSize := pagination(c)
	resp, err := h.svc.AdminListUserActivity(c.Request.Context(), adminID, c.Param("id"), page, pageSize)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminListPackagesV2 返回后台套餐配置。
func (h *Handler) AdminListPackagesV2(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	page, pageSize := pagination(c)
	resp, err := h.svc.AdminListPackagesV2(c.Request.Context(), adminID, c.Query("type"), page, pageSize)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminCreatePackageV2 创建后台套餐配置。
func (h *Handler) AdminCreatePackageV2(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.AdminCreatePackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.AdminCreatePackageV2(c.Request.Context(), adminID, req)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminUpdatePackageV2 更新后台套餐配置。
func (h *Handler) AdminUpdatePackageV2(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.AdminUpdatePackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.AdminUpdatePackageV2(c.Request.Context(), adminID, c.Param("id"), req)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminDeletePackageV2 删除后台套餐配置。
func (h *Handler) AdminDeletePackageV2(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	if err := h.svc.AdminDeletePackageV2(c.Request.Context(), adminID, c.Param("id")); err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// AdminListActionLogs 返回套餐审计日志。
func (h *Handler) AdminListActionLogs(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	page, pageSize := pagination(c)
	resp, err := h.svc.AdminListActionLogs(c.Request.Context(), adminID, c.Query("targetType"), c.Query("targetId"), c.Query("action"), page, pageSize)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}
