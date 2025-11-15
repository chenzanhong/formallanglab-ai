// internal/server/server.go
package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai/internal/handler"
	"ai/internal/service"
	"ai/logs"

	"github.com/redis/go-redis/v9"
)

type Server struct {
	httpServer *http.Server
	redis      *redis.Client
}

// NewServer 创建并返回一个新 Server 实例
func NewServer(aiService service.AIService, redisClient *redis.Client, port int) *Server {
	aiHandler := handler.NewAIHandler(aiService)
	router := handler.SetupRouter(aiHandler)

	return &Server{
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: router,
		},
		redis: redisClient,
	}
}

// Start 启动 HTTP 服务器并监听中断信号
func (s *Server) Start() error {
	// 启动 HTTP 服务器（非阻塞）
	go func() {
		logs.Sugar.Infof("Server starting on %s", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logs.Sugar.Errorw("HTTP server failed", "error", err)
		}
	}()

	// 优雅关闭，等待系统信号（SIGINT/SIGTERM）
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 收到信号后开始优雅关闭
	logs.Sugar.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. 关闭 HTTP 服务器
	if err := s.httpServer.Shutdown(ctx); err != nil {
		logs.Sugar.Errorw("HTTP server forced to shutdown", "error", err)
	} else {
		logs.Sugar.Info("HTTP server stopped gracefully")
	}

	// 2. 关闭 Redis 连接
	if err := s.redis.Close(); err != nil {
		logs.Sugar.Warnw("Failed to close Redis connection", "error", err)
	} else {
		logs.Sugar.Info("Redis connection closed")
	}

	return nil
}
