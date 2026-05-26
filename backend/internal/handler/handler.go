package handler

import (
	"errors"
	"net/http"
	"strconv"

	"image2/backend/internal/dto"
	"image2/backend/internal/middleware"
	"image2/backend/internal/response"
	"image2/backend/internal/security"
	"image2/backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Handler 汇总所有 HTTP 处理器。
type Handler struct {
	svc *service.Service
}

// New 创建处理器。
func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// JWT 暴露令牌管理器，供路由中间件使用。
func (h *Handler) JWT() *security.JWTManager {
	return h.svc.JWT()
}

// Redis 暴露 Redis 客户端，供路由层限流中间件复用。
func (h *Handler) Redis() *redis.Client {
	return h.svc.Redis()
}

// RequireActiveUser 在 Token 合法之后继续确认账号仍处于可用状态。
func (h *Handler) RequireActiveUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := middleware.CurrentUserID(c)
		if !ok {
			response.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}
		if err := h.svc.EnsureActiveUser(c.Request.Context(), userID); err != nil {
			h.writeServiceError(c, err)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireAdminRole 确保当前用户具备管理员权限。
func (h *Handler) RequireAdminRole() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := middleware.CurrentUserID(c)
		if !ok {
			response.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}
		if err := h.svc.RequireAdmin(c.Request.Context(), userID); err != nil {
			response.Forbidden(c, "无权访问此资源")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireCurrentSession 确保当前 token 版本仍然是该账号的最新版本。
func (h *Handler) RequireCurrentSession() gin.HandlerFunc {
	return middleware.RequireTokenVersionMatch(h.svc.TokenVersionView)
}

// Health 返回服务健康状态。
func (h *Handler) Health(c *gin.Context) {
	response.Success(c, gin.H{
		"status":  "ok",
		"service": "ai-image-saas-backend",
	})
}

// SendEmailCode 发送邮箱验证码。
func (h *Handler) SendEmailCode(c *gin.Context) {
	var req dto.SendEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}

	resp, err := h.svc.SendEmailCode(c.Request.Context(), req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

func (h *Handler) Captcha(c *gin.Context) {
	resp, err := h.svc.GenerateCaptcha(c.Request.Context())
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	response.Success(c, resp)
}

// Register 注册用户。
func (h *Handler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}

	resp, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// Login 用户登录。
func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}

	resp, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

func (h *Handler) CheckResetPasswordEmail(c *gin.Context) {
	var req dto.CheckResetPasswordEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.CheckResetPasswordEmail(c.Request.Context(), req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) CheckRegisterAvailability(c *gin.Context) {
	var req dto.CheckRegisterAvailabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.CheckRegisterAvailability(c.Request.Context(), req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.ResetPassword(c.Request.Context(), req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	response.Success(c, resp)
}

// CheckIn 用户每日签到。
func (h *Handler) CheckIn(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	resp, err := h.svc.CheckIn(c.Request.Context(), userID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// GetPoints 获取用户积分。
func (h *Handler) GetPoints(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	resp, err := h.svc.GetPoints(c.Request.Context(), userID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// GetMe 获取当前登录用户信息。
func (h *Handler) GetMe(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	resp, err := h.svc.GetMe(c.Request.Context(), userID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// UploadReferenceImage 上传参考图片。
func (h *Handler) UploadReferenceImage(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "请选择要上传的图片")
		return
	}

	resp, err := h.svc.UploadUserImage(c.Request.Context(), userID, fileHeader, c.PostForm("purpose"))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// ListPackages 获取套餐列表。
func (h *Handler) ListPackages(c *gin.Context) {
	response.Success(c, h.svc.ListPackages(c.Request.Context(), c.Query("type")))
}

// RedeemPoints 将积分兑换为福利次数。
func (h *Handler) RedeemPoints(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	var req dto.RedeemPointsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}

	resp, err := h.svc.RedeemPoints(c.Request.Context(), userID, req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// CreateImageTask 提交生图任务。
func (h *Handler) CreateImageTask(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	var req dto.CreateImageTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}

	resp, err := h.svc.CreateImageTask(c.Request.Context(), userID, req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// RetryImageTask 基于历史任务重新提交一条生图任务。
func (h *Handler) RetryImageTask(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	resp, err := h.svc.RetryImageTask(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// GetImageTask 获取任务详情。
func (h *Handler) GetImageTask(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	resp, err := h.svc.GetImageTask(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// ListImageTasks 获取任务列表。
func (h *Handler) ListImageTasks(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	resp, err := h.svc.ListImageTasks(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// AdminStats 返回后台统计概览。
func (h *Handler) AdminStats(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	resp, err := h.svc.AdminStats(c.Request.Context(), adminID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminListUsers 返回后台用户列表。
func (h *Handler) AdminListUsers(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	page, pageSize := pagination(c)
	resp, err := h.svc.AdminListUsers(c.Request.Context(), adminID, c.Query("keyword"), c.Query("status"), page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminUpdateUserStatus 启用或禁用用户。
func (h *Handler) AdminUpdateUserStatus(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.AdminUpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.AdminUpdateUserStatus(c.Request.Context(), adminID, c.Param("id"), req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminAdjustPoints 人工调整用户积分。
func (h *Handler) AdminAdjustPoints(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var req dto.AdminAdjustPointsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}
	resp, err := h.svc.AdminAdjustPoints(c.Request.Context(), adminID, c.Param("id"), req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminListImageTasks 返回全站任务列表。
func (h *Handler) AdminListImageTasks(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	page, pageSize := pagination(c)
	providerKeyID, _ := strconv.ParseUint(c.Query("providerKeyId"), 10, 64)
	resp, err := h.svc.AdminListImageTasks(c.Request.Context(), adminID, c.Query("status"), c.Query("keyword"), providerKeyID, page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminListContentAudits 返回内容审核日志。
func (h *Handler) AdminListContentAudits(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	page, pageSize := pagination(c)
	resp, err := h.svc.AdminListContentAudits(c.Request.Context(), adminID, c.Query("status"), page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminReviewContentAudit 复审内容审核记录。
func (h *Handler) AdminReviewContentAudit(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}

	var req dto.AdminReviewContentAuditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数不合法")
		return
	}

	resp, err := h.svc.AdminReviewContentAudit(c.Request.Context(), adminID, c.Param("id"), req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminListPackages 返回后台套餐配置。
func (h *Handler) AdminListPackages(c *gin.Context) {
	adminID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	page, pageSize := pagination(c)
	resp, err := h.svc.AdminListPackages(c.Request.Context(), adminID, c.Query("type"), page, pageSize)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminCreatePackage 创建后台套餐配置。
func (h *Handler) AdminCreatePackage(c *gin.Context) {
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
	resp, err := h.svc.AdminCreatePackage(c.Request.Context(), adminID, req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	response.Success(c, resp)
}

// AdminUpdatePackage 更新后台套餐配置。
func (h *Handler) AdminUpdatePackage(c *gin.Context) {
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
	resp, err := h.svc.AdminUpdatePackage(c.Request.Context(), adminID, c.Param("id"), req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	response.Success(c, resp)
}

func pagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	return page, pageSize
}

func (h *Handler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidVerificationCode):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrInvalidCaptcha):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrCaptchaAttemptsExceeded):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrVerificationCodeExpired):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrVerificationAttemptsExceeded):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrVerificationCodeTooFrequent):
		response.TooManyRequests(c, err.Error())
	case errors.Is(err, service.ErrVerificationCodeRateLimited):
		response.TooManyRequests(c, err.Error())
	case errors.Is(err, service.ErrEmailDailyLimitReached):
		response.TooManyRequests(c, err.Error())
	case errors.Is(err, service.ErrEmailAlreadyRegistered):
		response.Conflict(c, err.Error())
	case errors.Is(err, service.ErrUsernameAlreadyRegistered):
		response.Conflict(c, err.Error())
	case errors.Is(err, service.ErrAccountExists):
		response.Conflict(c, err.Error())
	case errors.Is(err, service.ErrInviteCodeInvalid):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrRegistrationDisabled):
		response.Forbidden(c, err.Error())
	case errors.Is(err, service.ErrInvalidCredentials):
		response.Unauthorized(c, err.Error())
	case errors.Is(err, service.ErrUserDisabled):
		response.Forbidden(c, err.Error())
	case errors.Is(err, service.ErrAlreadyCheckedIn):
		response.Conflict(c, err.Error())
	case errors.Is(err, service.ErrInsufficientPoints):
		response.Conflict(c, err.Error())
	case errors.Is(err, service.ErrInsufficientCredits):
		response.Conflict(c, err.Error())
	case errors.Is(err, service.ErrUserNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrTaskNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrTaskStillGenerating):
		response.Conflict(c, err.Error())
	case errors.Is(err, service.ErrImageGenerationDisabled):
		response.Forbidden(c, err.Error())
	case errors.Is(err, service.ErrImageNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrImageDownloadFailed):
		response.JSON(c, http.StatusBadGateway, response.CodeInternal, err.Error(), nil)
	case errors.Is(err, service.ErrAuditNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrPackageInvalid):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrPromptRequired):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrInvalidImageSize):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrProviderNotConfigured):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrEmailSenderNotConfigured):
		response.InternalError(c, err.Error())
	case errors.Is(err, service.ErrForbidden):
		response.Forbidden(c, err.Error())
	case errors.Is(err, service.ErrContentRejected):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrUploadFileRequired):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrUploadFileTooLarge):
		response.JSON(c, 413, 41301, err.Error(), nil)
	case errors.Is(err, service.ErrUploadUnsupportedType):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrUploadStorageNotConfig):
		response.InternalError(c, err.Error())
	case errors.Is(err, service.ErrInvalidReferenceImageURL):
		response.BadRequest(c, err.Error())
	default:
		response.InternalError(c, "服务端异常")
	}
}
