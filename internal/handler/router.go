package handler

import (
	"github.com/chenzanhong/goutil/jwtx"
	"github.com/gin-gonic/gin"

	"github.com/chenzanhong/formallanglab-ai/internal/middleware/cors"
	"github.com/chenzanhong/formallanglab-ai/internal/middleware/jwt"
	"github.com/chenzanhong/formallanglab-ai/internal/middleware/metrics"
	"github.com/chenzanhong/formallanglab-ai/internal/middleware/rate"
	"github.com/chenzanhong/formallanglab-ai/internal/middleware/requestid"
)

// SetupRouter 设置路由
func SetupRouter(aiHandler *AIHandler) *gin.Engine {
	router := gin.Default()
	// 1. 请求ID中间件
	router.Use(requestid.RequestID())
	// 2. CORS中间件
	router.Use(cors.CORSMiddleware())
	// 3. 指标收集
	router.Use(metrics.HTTPMiddleware())

	// 健康检查
	router.GET("/gdesign/ai/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})
	router.HEAD("/gdesign/ai/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})
	router.GET("/gdesign/ai/metrics", metrics.MetricsHandler())
	// WebSocket聊天接口，不经过JWT中间件，直接通过URL参数 token 验证
	router.GET("/gdesign/ai/ws", jwt.AuthWebsocket(), aiHandler.AIChatWS)

	router.POST("/gdesign/ai/sse", jwtx.GinJWTAuthMiddleware(), rate.UserRateLimitMiddleware(), aiHandler.AIChatSSE) // 流式AI聊天接口，SSE

	// 自定义AI模型相关接口
	authGroup := router.Group("/gdesign/ai", jwtx.GinJWTAuthMiddleware())
	{
		// 获取AI配置
		authGroup.GET("/config", aiHandler.GetAIConfig)
		// 添加自定义AI模型
		authGroup.POST("/models", aiHandler.AddCustomAIModel)
		// 获取自定义AI模型列表
		authGroup.GET("/models", aiHandler.GetCustomAIModels)
		// 更新自定义AI模型
		authGroup.PUT("/models/:id", aiHandler.UpdateCustomAIModel)
		// 删除自定义AI模型
		authGroup.DELETE("/models/:id", aiHandler.DeleteCustomAIModel)
		// 切换模型
		authGroup.POST("/switch", aiHandler.SwitchModel)
	}

	return router
}
