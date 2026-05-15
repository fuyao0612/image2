package handler

import (
	"errors"

	"image2/backend/internal/dto"
	"image2/backend/internal/middleware"
	"image2/backend/internal/response"
	"image2/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListPromptPlaza(c *gin.Context) {
	resp, err := h.svc.ListPromptPlaza(c.Request.Context())
	if err != nil {
		h.writePromptPlazaError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) ListPromptPlazaForUser(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	resp, err := h.svc.ListPromptPlazaForUser(c.Request.Context(), userID)
	if err != nil {
		h.writePromptPlazaError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) SubmitPromptPlaza(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.SubmitPromptPlazaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.SubmitPromptPlaza(c.Request.Context(), userID, req)
	if err != nil {
		h.writePromptPlazaError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) TogglePromptPlazaFavorite(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	resp, err := h.svc.TogglePromptPlazaFavorite(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		h.writePromptPlazaError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) ListMyPromptPlaza(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	resp, err := h.svc.ListMyPromptPlaza(c.Request.Context(), userID)
	if err != nil {
		h.writePromptPlazaError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminListPromptPlaza(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	page, pageSize := pagination(c)
	resp, err := h.svc.AdminListPromptPlaza(c.Request.Context(), adminID, c.Query("status"), page, pageSize)
	if err != nil {
		h.writePromptPlazaError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminCreatePromptPlaza(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.AdminPromptPlazaPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.AdminCreatePromptPlaza(c.Request.Context(), adminID, req)
	if err != nil {
		h.writePromptPlazaError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminUpdatePromptPlaza(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.AdminPromptPlazaPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.AdminUpdatePromptPlaza(c.Request.Context(), adminID, c.Param("id"), req)
	if err != nil {
		h.writePromptPlazaError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminReviewPromptPlaza(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.AdminReviewPromptPlazaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.AdminReviewPromptPlaza(c.Request.Context(), adminID, c.Param("id"), req)
	if err != nil {
		h.writePromptPlazaError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) AdminDeletePromptPlaza(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.DeletePromptPlazaRequest
	_ = c.ShouldBindJSON(&req)
	if err := h.svc.AdminDeletePromptPlaza(c.Request.Context(), adminID, c.Param("id"), req.Reason); err != nil {
		h.writePromptPlazaError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *Handler) writePromptPlazaError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrPromptPlazaInvalid):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrPromptPlazaNotFound):
		response.NotFound(c, err.Error())
	default:
		h.writeServiceErrorV2(c, err)
	}
}
