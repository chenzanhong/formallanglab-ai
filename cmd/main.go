package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/chenzanhong/goutil/jwtx"
	"github.com/chenzanhong/zlog"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	cf "github.com/chenzanhong/formallanglab-ai/configs"
	"github.com/chenzanhong/formallanglab-ai/internal/core"
	"github.com/chenzanhong/formallanglab-ai/internal/domain/model"
	"github.com/chenzanhong/formallanglab-ai/internal/middleware"
	mtr "github.com/chenzanhong/formallanglab-ai/internal/middleware/metrics"
	"github.com/chenzanhong/formallanglab-ai/internal/repository"
	"github.com/chenzanhong/formallanglab-ai/internal/server"
	"github.com/chenzanhong/formallanglab-ai/internal/service"
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

	// 2. 设置环境变量
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

	// 8. 初始化AI仓库
	aiRepo := repository.NewAIRepository(redisClient)

	// 9. 初始化QACache（异步加载，失败不影响服务启动）
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

	// 10. 初始化 OpenAI 客户端管理器（TTL 缓存，默认过期时间 30 分钟）
	clientManager := service.NewOpenAIClientManager(0)

	// 11. 预先初始化默认AI客户端（启动时验证配置是否正确）
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		zlog.Fatalf("DASHSCOPE_API_KEY is required")
	}
	baseURL := os.Getenv("DASHSCOPE_BASE_URL")
	if baseURL == "" {
		zlog.Fatalf("DASHSCOPE_BASE_URL is required")
	}
	// 预先创建默认客户端，后续使用时直接从缓存获取
	_, err = clientManager.GetDefaultClient(apiKey, baseURL)
	if err != nil {
		zlog.Fatalf("Failed to initialize default OpenAI client: %v", err)
	}
	zlog.Infow("Default OpenAI client initialized", "baseURL", baseURL)

	// 12. 创建AI服务
	aiService := service.NewAIService(clientManager, aiRepo, userAIRepo, qaCache, config.AI)

	// 13. 启动 pprof http 服务（通过 PPROF_PORT 环境变量控制，默认为 6060）
	go func() {
		if v, ok := os.LookupEnv("PPROF_PORT"); ok && v != "" && v != "0" {
			zlog.Info("Starting pprof on :" + v)
			if err := http.ListenAndServe(":"+v, nil); err != nil {
				zlog.Errorf("pprof server error: %v", err)
			}
		}
	}()

	// 14. 启动独立的 metrics 服务（通过 METRICS_PORT 环境变量控制）
	go func() {
		metricsPort := os.Getenv("METRICS_PORT")
		if metricsPort != "" && metricsPort != "0" {
			zlog.Infow("Starting metrics server on", "port", metricsPort)
			mux := http.NewServeMux()
			mux.Handle("/gdesign/ai/metrics", promhttp.Handler())
			if err := http.ListenAndServe(fmt.Sprintf(":%s", metricsPort), mux); err != nil {
				zlog.Errorf("Metrics server error: %v", err)
			}
		}
	}()

	server := server.NewServer(aiService, redisClient, config.Server.Port)
	server.Start()
}
