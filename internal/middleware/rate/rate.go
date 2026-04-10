package rate

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

const (
	userRate  = 5 // req/s
	userBurst = 10
)

var (
	userLimiters  sync.Map
	globalLimiter = rate.NewLimiter(rate.Limit(40), 100)
)

func GetLimiter(username string) *rate.Limiter {
	if limiter, ok := userLimiters.Load(username); ok {
		return limiter.(*rate.Limiter)
	}

	// 不存在则创建，已经经过JWT，不会被恶意刷次数
	limiter := rate.NewLimiter(rate.Limit(userRate), userBurst)

	actual, loaded := userLimiters.LoadOrStore(username, limiter)
	if loaded {
		return actual.(*rate.Limiter)
	}

	return limiter
}

func UserRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userStr, exists := c.Get("username")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization required"})
			c.Abort()

			return
		}
		username, ok := userStr.(string)
		if !ok || username == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid username in context"})
			c.Abort()

			return
		}

		limiter := GetLimiter(username)
		if !limiter.Allow() {
			c.Header("Retry-After", "1")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please slow down.",
			})
			c.Abort()

			return
		}
		c.Next()
	}
}

func GlobalRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !globalLimiter.Allow() {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Global rate limit exceeded. Please try again later.",
			})

			return
		}
		c.Next()
	}
}
