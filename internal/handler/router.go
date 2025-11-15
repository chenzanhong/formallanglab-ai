package handler

import (
	"ai/internal/middleware"

	mtr "ai/internal/middleware/metrics"

	"github.com/gin-gonic/gin"
)

// SetupRouter 设置路由
func SetupRouter(aiHandler *AIHandler) *gin.Engine {
	router := gin.Default()
	// WebSocket聊天接口，不经过JWT中间件，直接通过URL参数 token 验证
	router.GET("/gdesign/ai/ws", middleware.AuthWebsocket(), aiHandler.AIChatWS)

	// 1. 请求ID中间件
	router.Use(middleware.RequestID())
	// 2. CORS中间件
	router.Use(middleware.CORSMiddleware())
	// 3. 指标收集
	router.Use(mtr.HTTPMiddleware())

	// 健康检查
	router.GET("/gdesign/ai/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	router.GET("/gdesign/ai/metrics", mtr.MetricsHandler())
	router.POST("/gdesign/ai/sse", middleware.JWTAuthMiddleware(), middleware.UserRateLimitMiddleware(), aiHandler.AIChatSSE) // 流式AI聊天接口，SSE

	return router
}
