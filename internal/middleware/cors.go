package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware 是一个中间件，用于配置 CORS
func CORSMiddleware() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{
		"https://caohaitong.xyz",
		"http://caohaitong.xyz",
		"http://113.44.170.52",
		"https://113.44.170.52",
		"http://localhost:5173",
		"http://localhost:3000",
	}

	config.AllowMethods = []string{"*"}
	config.AllowHeaders = []string{"*"}

	return cors.New(config)
}
