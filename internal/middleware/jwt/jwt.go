package jwt

import (
	"net/http"
	"sync"

	"github.com/chenzanhong/goutil/jwtx"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Username string `json:"username" inject:"username"`
	UserID   int64  `json:"user_id"       inject:"user_id"`
	jwtx.RegisteredClaims
}

var (
	jwtKey []byte
	done   sync.Once
)

func SetJWTKey(key string) {
	done.Do(func() {
		jwtKey = []byte(key)
	})
}

// AuthWebsocket 一个用于 WS 的轻量 JWT 中间件
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

		c.Set("username", claims.Username)
		c.Set("user_id", claims.UserID)
		c.Next()
	}
}
