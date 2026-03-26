package middleware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var jwtKey []byte
var done sync.Once

func SetJWTKey(key string) {
	done.Do(func() {
		jwtKey = []byte(key)
	})
}

// 1. 定义一个只用于提取 username 的轻量 JWT 中间件
func AuthWebsocket() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.Query("token")
		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			c.Abort()
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		// 把用户名和用户 ID 存入上下文，供 handler 使用
		c.Set("username", claims.Username)
		c.Set("user_id", claims.UserID)
		c.Next() // 继续执行 handler（即你的 AIChatWS）
	}
}
