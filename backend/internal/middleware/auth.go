package middleware

import (
	"context"
	"strings"

	"image2/backend/internal/response"
	"image2/backend/internal/security"
	"image2/backend/internal/service"

	"github.com/gin-gonic/gin"
)

const userIDContextKey = "auth_user_id"
const userTokenVersionContextKey = "auth_token_version"

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

		userID, tokenVersion, err := jwtManager.Parse(token)
		if err != nil {
			response.Unauthorized(c, "登录已过期，请重新登录")
			c.Abort()
			return
		}

		c.Set(userIDContextKey, userID)
		c.Set(userTokenVersionContextKey, tokenVersion)
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

// CurrentTokenVersion 读取当前 token 版本。
func CurrentTokenVersion(c *gin.Context) (int64, bool) {
	value, ok := c.Get(userTokenVersionContextKey)
	if !ok {
		return 0, false
	}

	tokenVersion, ok := value.(int64)
	return tokenVersion, ok
}

// RequireTokenVersionMatch 确保 token 版本与数据库中的当前版本一致。
func RequireTokenVersionMatch(loader func(ctx context.Context, userID uint64) (service.TokenVersionView, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := CurrentUserID(c)
		if !ok {
			response.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}

		tokenVersion, ok := CurrentTokenVersion(c)
		if !ok {
			response.Unauthorized(c, "登录已过期，请重新登录")
			c.Abort()
			return
		}

		view, err := loader(c.Request.Context(), userID)
		if err != nil {
			response.Unauthorized(c, "登录已过期，请重新登录")
			c.Abort()
			return
		}
		if view.TokenVersion != tokenVersion {
			response.Unauthorized(c, "密码已更新，请重新登录")
			c.Abort()
			return
		}

		c.Next()
	}
}
