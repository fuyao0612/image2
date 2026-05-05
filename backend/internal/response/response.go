package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 统一的业务返回码。
const (
	CodeOK              = 0
	CodeInvalidArgument = 40001
	CodeUnauthorized    = 40101
	CodeForbidden       = 40301
	CodeNotFound        = 40401
	CodeConflict        = 40901
	CodeTooManyRequests = 42901
	CodeInternal        = 50000
)

// Envelope 是所有 API 的统一响应封装。
type Envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// JSON 统一输出响应。
func JSON(c *gin.Context, httpStatus int, code int, message string, data any) {
	c.JSON(httpStatus, Envelope{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// Success 表示通用成功返回。
func Success(c *gin.Context, data any) {
	JSON(c, http.StatusOK, CodeOK, "success", data)
}

// BadRequest 表示参数错误。
func BadRequest(c *gin.Context, message string) {
	JSON(c, http.StatusBadRequest, CodeInvalidArgument, message, nil)
}

// Unauthorized 表示未登录或 token 无效。
func Unauthorized(c *gin.Context, message string) {
	JSON(c, http.StatusUnauthorized, CodeUnauthorized, message, nil)
}

// Forbidden 表示无权限。
func Forbidden(c *gin.Context, message string) {
	JSON(c, http.StatusForbidden, CodeForbidden, message, nil)
}

// NotFound 表示资源不存在。
func NotFound(c *gin.Context, message string) {
	JSON(c, http.StatusNotFound, CodeNotFound, message, nil)
}

// Conflict 表示资源冲突。
func Conflict(c *gin.Context, message string) {
	JSON(c, http.StatusConflict, CodeConflict, message, nil)
}

// TooManyRequests 表示请求过于频繁。
func TooManyRequests(c *gin.Context, message string) {
	JSON(c, http.StatusTooManyRequests, CodeTooManyRequests, message, nil)
}

// InternalError 表示服务端异常。
func InternalError(c *gin.Context, message string) {
	JSON(c, http.StatusInternalServerError, CodeInternal, message, nil)
}
