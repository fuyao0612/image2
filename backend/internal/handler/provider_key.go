package handler

import (
	"image2/backend/internal/dto"
	"image2/backend/internal/middleware"
	"image2/backend/internal/response"

	"github.com/gin-gonic/gin"
)

// AdminListProviderKeys 返回后台生图 Key 池列表。
func (h *Handler) AdminListProviderKeys(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	page, pageSize := pagination(c)
	resp, err := h.svc.AdminListProviderKeys(c.Request.Context(), adminID, page, pageSize)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminCreateProviderKey 新建后台生图 Key。
func (h *Handler) AdminCreateProviderKey(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.AdminProviderKeyPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.AdminCreateProviderKey(c.Request.Context(), adminID, req)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminUpdateProviderKey 更新后台生图 Key。
func (h *Handler) AdminUpdateProviderKey(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.AdminProviderKeyPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.AdminUpdateProviderKey(c.Request.Context(), adminID, c.Param("id"), req)
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminDeleteProviderKey 删除后台生图 Key。
func (h *Handler) AdminDeleteProviderKey(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	if err := h.svc.AdminDeleteProviderKey(c.Request.Context(), adminID, c.Param("id")); err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// AdminUseProviderKey 将指定 Key 设为当前使用中。
func (h *Handler) AdminUseProviderKey(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	resp, err := h.svc.AdminUseProviderKey(c.Request.Context(), adminID, c.Param("id"))
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}
