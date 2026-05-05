package handler

import (
	"image2/backend/internal/response"

	"github.com/gin-gonic/gin"
)

// GetBillingSettings 返回前端需要展示的公开计费规则。
func (h *Handler) GetBillingSettings(c *gin.Context) {
	resp, err := h.svc.BillingSettings(c.Request.Context())
	if err != nil {
		h.writeServiceErrorV2(c, err)
		return
	}
	response.Success(c, resp)
}
