package handler

import (
	"strconv"

	"image2/backend/internal/middleware"
	"image2/backend/internal/response"

	"github.com/gin-gonic/gin"
)

// ListPointsLogs 返回当前用户的积分流水分页。
func (h *Handler) ListPointsLogs(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	page, pageSize := logPagination(c, 7)
	resp, err := h.svc.ListPointsLogs(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// ListCreditLogs 返回当前用户的福利次数流水分页。
func (h *Handler) ListCreditLogs(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	page, pageSize := logPagination(c, 7)
	resp, err := h.svc.ListCreditLogs(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// ListUserActivity 返回当前用户的统一记录流分页。
func (h *Handler) ListUserActivity(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	page, pageSize := logPagination(c, 10)
	resp, err := h.svc.ListUserActivity(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

func logPagination(c *gin.Context, defaultPageSize int) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", strconv.Itoa(defaultPageSize)))
	return page, pageSize
}
