package main

import (
	cf "ai/configs"
	"ai/internal/core"
	"ai/internal/domain/model"
	"ai/internal/middleware"
	mtr "ai/internal/middleware/metrics"
	"ai/internal/repository"
	"ai/internal/server"
	"ai/internal/service"
	"log"
	"net/http"
	"os"

	"github.com/chenzanhong/goutil/jwtx"
	"github.com/chenzanhong/zlog"
)

func init() {
	mtr.PrometheusRegister()  // 初始化Prometheus
	core.RegisterValidators() // 注册自定义验证器
}

func main() {
	// 1. 加载配置
	config, err := cf.LoadConfig()
	if err != nil {
		log.Fatalf("加载配置失败：%v", err.Error())
	}

	// 2. 设置环境变量，确保未有的环境变量有值
	cf.SyncConfigToEnv(config)

	// 3. 设置 JWT 密钥
	middleware.SetJWTKey(config.JWT.Key)

	// 4. 初始化日志
	zlog.InitLogger(config.Log)

	// 使用环境变量中的 JWT key，确保与 auth 服务一致
	jwtKey := os.Getenv("JWT_KEY")
	if jwtKey == "" {
		jwtKey = config.JWT.Key
	}
	jwtx.InitWithHS256(jwtKey, &middleware.Claims{}, jwtx.WithAutoInject(true))

	// 5. 初始化Redis
	redisClient, err := repository.InitRedis()
	if err != nil {
		zlog.Fatalf("Failed to initialize Redis: %v", err)
	}

	// 6. 初始化数据库
	db, err := repository.InitDB()
	if err != nil {
		zlog.Fatalf("Failed to initialize database: %v", err)
	}

	// 7. 初始化用户仓库
	userAIRepo := repository.NewUserAIRepository(db, redisClient)

	// // 8. 初始化OpenAI客户端
	// apiKey := os.Getenv("DASHSCOPE_API_KEY")
	// if apiKey == "" {
	// 	zlog.Fatalf("DASHSCOPE_API_KEY is required")
	// }
	// baseURL := os.Getenv("DASHSCOPE_BASE_URL")
	// if baseURL == "" {
	// 	zlog.Fatalf("DASHSCOPE_BASE_URL is required")
	// }
	// openaiClient, err := service.NewOpenAIClient(apiKey, baseURL)
	// if err != nil {
	// 	log.Fatalf("Failed to create OpenAI client: %v", err)
	// }
	// 8. 初始化AI仓库
	aiRepo := repository.NewAIRepository(redisClient)

	// 7. 初始化QACache
	qaCache := model.NewQACache()
	qaCacheLoader := repository.NewQACacheLoader()
	go func() {
		if err = qaCacheLoader.LoadCache(qaCache, os.Getenv("QA_CACHE_DIR")); err != nil {
			zlog.Errorw("Failed to load QA cache, running without cache", "error", err)
			// 不 fatal，允许服务启动（只是缓存未命中）
		} else {
			zlog.Infow("QA cache loaded successfully")
		}
	}()

	// 8. 初始化OpenAI客户端管理器
	clientManager := service.NewOpenAIClientManager()

	// 9. 预先初始化默认AI客户端
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		zlog.Fatalf("DASHSCOPE_API_KEY is required")
	}
	baseURL := os.Getenv("DASHSCOPE_BASE_URL")
	if baseURL == "" {
		zlog.Fatalf("DASHSCOPE_BASE_URL is required")
	}
	// 预先创建默认客户端，后续使用时直接从缓存获取
	_ = clientManager.GetClient(apiKey, baseURL)
	zlog.Infow("Default OpenAI client initialized", "baseURL", baseURL)

	// 10. 创建AI服务
	aiService := service.NewAIService(clientManager, aiRepo, userAIRepo, qaCache, config.AI)
	// 启动pprof http服务
	go func() {
		zlog.Info("Starting pprof on localhost:" + os.Getenv("PPROF_PORT"))
		http.ListenAndServe("localhost:"+os.Getenv("PPROF_PORT"), nil)
	}()
	server := server.NewServer(aiService, redisClient, config.Server.Port)
	server.Start()
}
