package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"image2/backend/internal/cache"
	"image2/backend/internal/config"
	"image2/backend/internal/database"
	"image2/backend/internal/handler"
	"image2/backend/internal/router"
	"image2/backend/internal/service"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("配置校验失败: %v", err)
	}

	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	redisClient, err := cache.OpenRedis(cfg)
	if err != nil {
		log.Fatalf("初始化 Redis 失败: %v", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	svc := service.New(cfg, db, redisClient)
	settingsCtx, settingsCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := svc.EnsureSystemSettings(settingsCtx); err != nil {
		settingsCancel()
		log.Fatalf("初始化系统配置失败: %v", err)
	}
	settingsCancel()

	recoverCtx, recoverCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := svc.RecoverPendingImageTasks(recoverCtx); err != nil {
		recoverCancel()
		log.Fatalf("恢复待处理生图任务失败: %v", err)
	}
	recoverCancel()

	maintenanceCtx, maintenanceCancel := context.WithCancel(context.Background())
	svc.StartImageTaskMaintenance(maintenanceCtx)
	svc.StartDailyCreditSettlementMaintenance(maintenanceCtx)

	h := handler.New(svc)
	engine := router.New(cfg, h)

	server := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      engine,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("后端服务已启动，监听地址: %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	maintenanceCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("优雅关闭失败: %v", err)
	}
}
