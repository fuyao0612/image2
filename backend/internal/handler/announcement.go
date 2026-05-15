package handler

import (
	"errors"

	"image2/backend/internal/dto"
	"image2/backend/internal/middleware"
	"image2/backend/internal/response"
	"image2/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListAnnouncements(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	resp, err := h.svc.ListAnnouncements(c.Request.Context(), userID)
	if err != nil {
		h.writeAnnouncementError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) MarkAnnouncementRead(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	resp, err := h.svc.MarkAnnouncementRead(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		h.writeAnnouncementError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminListAnnouncements(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	page, pageSize := pagination(c)
	resp, err := h.svc.AdminListAnnouncements(c.Request.Context(), adminID, page, pageSize)
	if err != nil {
		h.writeAnnouncementError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminCreateAnnouncement(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.AdminCreateAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.AdminCreateAnnouncement(c.Request.Context(), adminID, req)
	if err != nil {
		h.writeAnnouncementError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminUpdateAnnouncement(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.AdminUpdateAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.AdminUpdateAnnouncement(c.Request.Context(), adminID, c.Param("id"), req)
	if err != nil {
		h.writeAnnouncementError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminUploadAnnouncementImage(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "请选择要上传的图片")
		return
	}
	var resp dto.UploadImageResponse
	switch c.PostForm("purpose") {
	case "prompt-plaza", "prompt-plaza-preview":
		resp, err = h.svc.AdminUploadPromptPlazaImage(c.Request.Context(), adminID, fileHeader)
	case "product-image":
		resp, err = h.svc.AdminUploadProductImage(c.Request.Context(), adminID, fileHeader)
	default:
		resp, err = h.svc.AdminUploadAnnouncementImage(c.Request.Context(), adminID, fileHeader)
	}
	if err != nil {
		h.writeAnnouncementError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminDeleteAnnouncement(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	if err := h.svc.AdminDeleteAnnouncement(c.Request.Context(), adminID, c.Param("id")); err != nil {
		h.writeAnnouncementError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *Handler) writeAnnouncementError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrAnnouncementInvalid):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrAnnouncementNotFound):
		response.NotFound(c, err.Error())
	default:
		h.writeServiceErrorV2(c, err)
	}
}
