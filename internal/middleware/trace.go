package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDKey 请求ID上下文键
const (
	RequestIDKey = "request_id"
)

// RequestID 请求ID中间件（轻量级版本）
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 尝试从请求头获取
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// 创建上下文
		ctx := context.WithValue(c.Request.Context(), RequestIDKey, requestID)
		c.Request = c.Request.WithContext(ctx)

		// 设置响应头
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}
