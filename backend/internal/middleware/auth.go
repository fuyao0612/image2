package middleware

import (
	"strings"

	"image2/backend/internal/response"
	"image2/backend/internal/security"

	"github.com/gin-gonic/gin"
)

const userIDContextKey = "auth_user_id"

// RequireAuth 校验 Bearer Token，并把用户 ID 写入上下文。
func RequireAuth(jwtManager *security.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if header == "" || !strings.HasPrefix(strings.ToLower(header), "bearer ") {
			response.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}

		token := strings.TrimSpace(header[len("Bearer "):])
		if token == "" {
			response.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}

		userID, err := jwtManager.Parse(token)
		if err != nil {
			response.Unauthorized(c, "登录已过期，请重新登录")
			c.Abort()
			return
		}

		c.Set(userIDContextKey, userID)
		c.Next()
	}
}

// CurrentUserID 读取当前登录用户 ID。
func CurrentUserID(c *gin.Context) (uint64, bool) {
	value, ok := c.Get(userIDContextKey)
	if !ok {
		return 0, false
	}

	userID, ok := value.(uint64)
	return userID, ok
}
