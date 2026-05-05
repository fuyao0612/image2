package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"image2/backend/internal/middleware"
	"image2/backend/internal/response"

	"github.com/gin-gonic/gin"
)

// DownloadImageTaskImage 下载当前用户某个生图任务中的指定图片。
func (h *Handler) DownloadImageTaskImage(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	imageIndex, err := strconv.Atoi(c.DefaultQuery("index", "0"))
	if err != nil || imageIndex < 0 {
		response.BadRequest(c, "图片序号不合法")
		return
	}

	data, _, fileName, err := h.svc.DownloadImageTaskImage(c.Request.Context(), userID, c.Param("id"), imageIndex)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.DataFromReader(http.StatusOK, int64(len(data)), "application/octet-stream", bytes.NewReader(data), map[string]string{
		"Content-Disposition":    attachmentHeader(fileName),
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "no-store, max-age=0",
	})
}

func attachmentHeader(fileName string) string {
	safeName := sanitizeDownloadFileName(fileName)
	encodedName := url.PathEscape(safeName)
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, safeName, encodedName)
}

func sanitizeDownloadFileName(fileName string) string {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return "image2.png"
	}

	replacer := strings.NewReplacer("\\", "_", "/", "_", "\r", "_", "\n", "_", `"`, "_", ";", "_")
	return replacer.Replace(fileName)
}
