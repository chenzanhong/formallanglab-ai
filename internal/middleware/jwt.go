package middleware

import (
	"github.com/chenzanhong/goutil/jwtx"
)

type Claims struct {
	Username string `json:"username" inject:"username"` // inject 到 gin.Context 的 key
	UserID   int64  `json:"user_id"       inject:"user_id"`
	jwtx.RegisteredClaims
}
