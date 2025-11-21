package middleware

import (
	"github.com/chenzanhong/goutil/jwtx"
	"github.com/gin-gonic/gin"
)

type Claims struct {
	Username string `json:"username" inject:"username"` // inject 到 gin.Context 的 key
	UserID   int64  `json:"id"       inject:"user_id"`
	jwtx.RegisteredClaims
}

var JWTAuthMiddleware func() gin.HandlerFunc
