package router

import (
	"strings"
	"time"

	"image2/backend/internal/config"
	"image2/backend/internal/handler"
	"image2/backend/internal/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// New 构建 Gin 路由。
func New(cfg config.Config, h *handler.Handler) *gin.Engine {
	if strings.EqualFold(cfg.AppEnv, "prod") {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())
	engine.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSAllowOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	engine.GET("/healthz", h.Health)
	if strings.EqualFold(cfg.ImageStorageDriver, "local") && strings.TrimSpace(cfg.ImageStorageLocalDir) != "" {
		publicPath := strings.TrimSpace(cfg.ImageStoragePublicPath)
		if publicPath == "" {
			publicPath = "/uploads"
		}
		engine.Static("/"+strings.Trim(publicPath, "/"), cfg.ImageStorageLocalDir)
	}

	limiter := middleware.NewRateLimiter(cfg.RateLimitEnabled, h.Redis())

	api := engine.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.GET("/captcha", limiter.LimitIP("auth_captcha", cfg.RateLimitRegisterPerHour, time.Hour), h.Captcha)
			auth.POST(
				"/email-code",
				limiter.LimitEmail("auth_email_code_minute", cfg.RateLimitEmailPerMinute, time.Minute),
				limiter.LimitEmail("auth_email_code_hour", cfg.RateLimitEmailPerHour, time.Hour),
				h.SendEmailCode,
			)
			auth.POST("/register", limiter.LimitIP("auth_register", cfg.RateLimitRegisterPerHour, time.Hour), h.Register)
			auth.POST("/login", limiter.LimitIP("auth_login", cfg.RateLimitLoginPerMinute, time.Minute), h.Login)
			auth.POST("/register/check", limiter.LimitIP("auth_register_check", cfg.RateLimitLoginPerMinute, time.Minute), h.CheckRegisterAvailability)
			auth.POST("/reset-password/check", limiter.LimitIP("auth_reset_password_check", cfg.RateLimitLoginPerMinute, time.Minute), h.CheckResetPasswordEmail)
			auth.POST("/reset-password", limiter.LimitEmail("auth_reset_password", cfg.RateLimitEmailPerHour, time.Hour), h.ResetPassword)
		}

		api.GET("/packages", h.ListPackages)
		api.GET("/billing-settings", h.GetBillingSettings)
		api.GET("/prompt-plaza", h.ListPromptPlaza)
		api.GET("/products", h.ListProducts)
		api.GET("/product-categories", h.ListProductCategories)

		user := api.Group("/user")
		user.Use(middleware.RequireAuth(h.JWT()))
		user.Use(h.RequireCurrentSession())
		user.Use(h.RequireActiveUser())
		{
			user.GET("/me", h.GetMe)
			user.GET("/invite", h.GetInviteInfo)
			user.PATCH("/password", h.ChangePassword)
			user.POST("/checkin", limiter.LimitUser("user_checkin", cfg.RateLimitCheckinPerMinute, time.Minute), h.CheckIn)
			user.GET("/points", h.GetPoints)
			user.GET("/points-logs", h.ListPointsLogs)
			user.GET("/credit-logs", h.ListCreditLogs)
			user.GET("/logs", h.ListUserActivity)
			user.GET("/announcements", h.ListAnnouncements)
			user.POST("/announcements/:id/read", h.MarkAnnouncementRead)
			user.GET("/prompt-plaza", h.ListMyPromptPlaza)
			user.GET("/prompt-plaza-gallery", h.ListPromptPlazaForUser)
			user.POST("/prompt-plaza", limiter.LimitUser("prompt_plaza_submit_hour", cfg.RateLimitPromptSubmitPerHour, time.Hour), h.SubmitPromptPlaza)
			user.POST("/prompt-plaza/:id/favorite", h.TogglePromptPlazaFavorite)
		}

		uploads := api.Group("/uploads")
		uploads.Use(middleware.RequireAuth(h.JWT()))
		uploads.Use(h.RequireCurrentSession())
		uploads.Use(h.RequireActiveUser())
		{
			uploads.POST("/reference-image", limiter.LimitUser("upload_reference_image", cfg.RateLimitImagePerMinute, time.Minute), h.UploadReferenceImage)
		}

		tasks := api.Group("/image-tasks")
		tasks.Use(middleware.RequireAuth(h.JWT()))
		tasks.Use(h.RequireCurrentSession())
		tasks.Use(h.RequireActiveUser())
		{
			tasks.POST("", limiter.LimitUser("image_task_create", cfg.RateLimitImagePerMinute, time.Minute), h.CreateImageTask)
			tasks.GET("", h.ListImageTasks)
			tasks.POST("/:id/retry", limiter.LimitUser("image_task_retry", cfg.RateLimitImagePerMinute, time.Minute), h.RetryImageTask)
			tasks.GET("/:id/images/download", h.DownloadImageTaskImage)
			tasks.GET("/:id", h.GetImageTask)
		}

		redeem := api.Group("/redeem-code")
		redeem.Use(middleware.RequireAuth(h.JWT()))
		redeem.Use(h.RequireCurrentSession())
		redeem.Use(h.RequireActiveUser())
		{
			redeem.POST("", limiter.LimitUser("redeem_code", cfg.RateLimitPurchasePerMinute, time.Minute), h.RedeemCode)
		}

		admin := api.Group("/admin")
		admin.Use(middleware.RequireAuth(h.JWT()))
		admin.Use(h.RequireCurrentSession())
		admin.Use(h.RequireActiveUser())
		admin.Use(h.RequireAdminRole())
		{
			admin.GET("/stats", h.AdminStats)
			admin.GET("/users", h.AdminListUsers)
			admin.PATCH("/users/:id/status", h.AdminUpdateUserStatus)
			admin.POST("/users/:id/points-adjustments", h.AdminAdjustPoints)
			admin.GET("/users/:id/logs", h.AdminListUserActivity)
			admin.GET("/image-tasks", h.AdminListImageTasks)
			admin.GET("/redeem-codes", h.AdminListRedeemCodes)
			admin.POST("/redeem-codes", h.AdminCreateRedeemCodes)
			admin.DELETE("/redeem-codes/:id", h.AdminDeleteRedeemCode)
			admin.GET("/content-audits", h.AdminListContentAudits)
			admin.PATCH("/content-audits/:id/review", h.AdminReviewContentAudit)
			admin.GET("/billing-settings", h.AdminGetBillingSettings)
			admin.PATCH("/billing-settings", h.AdminUpdateBillingSettings)
			admin.GET("/packages", h.AdminListPackagesV2)
			admin.POST("/packages", h.AdminCreatePackageV2)
			admin.PATCH("/packages/:id", h.AdminUpdatePackageV2)
			admin.DELETE("/packages/:id", h.AdminDeletePackageV2)
			admin.GET("/provider-keys", h.AdminListProviderKeys)
			admin.POST("/provider-keys", h.AdminCreateProviderKey)
			admin.POST("/provider-keys/:id/use", h.AdminUseProviderKey)
			admin.PATCH("/provider-keys/:id", h.AdminUpdateProviderKey)
			admin.DELETE("/provider-keys/:id", h.AdminDeleteProviderKey)
			admin.GET("/announcements", h.AdminListAnnouncements)
			admin.POST("/announcements", h.AdminCreateAnnouncement)
			admin.POST("/announcements/upload-image", h.AdminUploadAnnouncementImage)
			admin.PATCH("/announcements/:id", h.AdminUpdateAnnouncement)
			admin.DELETE("/announcements/:id", h.AdminDeleteAnnouncement)
			admin.GET("/prompt-plaza", h.AdminListPromptPlaza)
			admin.POST("/prompt-plaza", h.AdminCreatePromptPlaza)
			admin.PATCH("/prompt-plaza/:id", h.AdminUpdatePromptPlaza)
			admin.PATCH("/prompt-plaza/:id/review", h.AdminReviewPromptPlaza)
			admin.DELETE("/prompt-plaza/:id", h.AdminDeletePromptPlaza)
			admin.GET("/products", h.AdminListProducts)
			admin.POST("/products", h.AdminCreateProduct)
			admin.PATCH("/products/:id", h.AdminUpdateProduct)
			admin.DELETE("/products/:id", h.AdminDeleteProduct)
			admin.GET("/product-categories", h.AdminListProductCategories)
			admin.POST("/product-categories", h.AdminCreateProductCategory)
			admin.PATCH("/product-categories/:id", h.AdminUpdateProductCategory)
			admin.DELETE("/product-categories/:id", h.AdminDeleteProductCategory)
			admin.GET("/action-logs", h.AdminListActionLogs)
			admin.POST("/data-cleanup/preview", h.AdminPreviewDataCleanup)
			admin.POST("/data-cleanup", h.AdminExecuteDataCleanup)
		}
	}

	return engine
}
