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
			auth.POST("/email-code", limiter.LimitIP("auth_email_code", cfg.RateLimitRegisterPerHour, time.Hour), h.SendEmailCode)
			auth.POST("/register", limiter.LimitIP("auth_register", cfg.RateLimitRegisterPerHour, time.Hour), h.Register)
			auth.POST("/login", limiter.LimitIP("auth_login", cfg.RateLimitLoginPerMinute, time.Minute), h.Login)
		}

		api.GET("/packages", h.ListPackages)
		api.GET("/billing-settings", h.GetBillingSettings)

		user := api.Group("/user")
		user.Use(middleware.RequireAuth(h.JWT()))
		{
			user.GET("/me", h.GetMe)
			user.PATCH("/password", h.ChangePassword)
			user.POST("/checkin", limiter.LimitUser("user_checkin", cfg.RateLimitCheckinPerMinute, time.Minute), h.CheckIn)
			user.GET("/points", h.GetPoints)
			user.GET("/points-logs", h.ListPointsLogs)
			user.GET("/credit-logs", h.ListCreditLogs)
			user.GET("/logs", h.ListUserActivity)
		}

		uploads := api.Group("/uploads")
		uploads.Use(middleware.RequireAuth(h.JWT()))
		{
			uploads.POST("/reference-image", limiter.LimitUser("upload_reference_image", cfg.RateLimitImagePerMinute, time.Minute), h.UploadReferenceImage)
		}

		tasks := api.Group("/image-tasks")
		tasks.Use(middleware.RequireAuth(h.JWT()))
		{
			tasks.POST("", limiter.LimitUser("image_task_create", cfg.RateLimitImagePerMinute, time.Minute), h.CreateImageTask)
			tasks.GET("", h.ListImageTasks)
			tasks.POST("/:id/retry", limiter.LimitUser("image_task_retry", cfg.RateLimitImagePerMinute, time.Minute), h.RetryImageTask)
			tasks.GET("/:id/images/download", h.DownloadImageTaskImage)
			tasks.GET("/:id", h.GetImageTask)
		}

		orders := api.Group("/recharge-orders")
		orders.Use(middleware.RequireAuth(h.JWT()))
		{
			orders.POST("", limiter.LimitUser("recharge_order_create", cfg.RateLimitPurchasePerMinute, time.Minute), h.CreateRechargeOrder)
			orders.GET("", h.ListRechargeOrders)
			orders.GET("/:orderNo", h.GetRechargeOrder)
			orders.POST("/:orderNo/confirm", limiter.LimitUser("recharge_order_confirm", cfg.RateLimitPurchasePerMinute, time.Minute), h.ConfirmRechargeOrder)
			orders.POST("/mock-pay", limiter.LimitUser("mock_purchase", cfg.RateLimitPurchasePerMinute, time.Minute), h.MockPurchase)
		}

		payments := api.Group("/payments")
		{
			payments.POST("/recharge/callback", h.RechargePaymentCallback)
		}

		admin := api.Group("/admin")
		admin.Use(middleware.RequireAuth(h.JWT()))
		{
			admin.GET("/stats", h.AdminStats)
			admin.GET("/users", h.AdminListUsers)
			admin.PATCH("/users/:id/status", h.AdminUpdateUserStatus)
			admin.POST("/users/:id/points-adjustments", h.AdminAdjustPoints)
			admin.GET("/users/:id/logs", h.AdminListUserActivity)
			admin.GET("/image-tasks", h.AdminListImageTasks)
			admin.GET("/recharge-orders", h.AdminListRechargeOrdersV2)
			admin.GET("/content-audits", h.AdminListContentAudits)
			admin.PATCH("/content-audits/:id/review", h.AdminReviewContentAudit)
			admin.GET("/billing-settings", h.AdminGetBillingSettings)
			admin.PATCH("/billing-settings", h.AdminUpdateBillingSettings)
			admin.GET("/packages", h.AdminListPackagesV2)
			admin.POST("/packages", h.AdminCreatePackageV2)
			admin.PATCH("/packages/:id", h.AdminUpdatePackageV2)
			admin.DELETE("/packages/:id", h.AdminDeletePackageV2)
			admin.GET("/action-logs", h.AdminListActionLogs)
		}
	}

	return engine
}
