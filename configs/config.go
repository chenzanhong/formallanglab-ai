package configs

import (
	// "ai/internal/middleware"

	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/chenzanhong/zlog"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Port int `yaml:"port"`
}

type RedisConfig struct {
	Host        string `yaml:"host"`
	Port        string `yaml:"port"`
	Password    string `yaml:"password"`
	DB          int    `yaml:"db"`
	MaxConn     int    `yaml:"max_conn"`
	MaxIdleConn int    `yaml:"max_idle_conn"`
}

type RateConfig struct {
	UserRate  int `yaml:"user_rate"`
	UserBurst int `yaml:"user_burst"`
}

type JWTConfig struct {
	Key string `yaml:"key"`
}

type AIConfig struct {
	DashscopeAPIKey      string `yaml:"dashscope_api_key"`
	DashscopeBaseURL     string `yaml:"dashscope_base_url"`
	DashscopeModel       string `yaml:"dashscope_model"`
	SessionExpireSeconds int    `yaml:"session_expire_seconds"`
	MaxSessionTurns      int    `yaml:"max_session_turns"` // 最大对话记录数
	MaxCtxToken          int    `yaml:"max_ctx_token"`
	// ChromaURL      string `yaml:"chroma_url"`
	// ChromaCollection string `yaml:"chroma_collection"`
}

type LogConfig struct {
	Level      string `yaml:"level"`
	Output     string `yaml:"output"` // "console", "file", "both"
	Format     string `yaml:"format"` // "json", "console" (只对终端输出生效)
	FilePath   string `yaml:"file_path"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
	Compress   bool   `yaml:"compress"`
	Sampling   bool   `yaml:"sampling"`
}

type QACacheConfig struct {
	Path     string `yaml:"path"`
	UseDB    bool   `yaml:"use_db"`    // 是否使用数据库存储
	LoadFromFile bool `yaml:"load_from_file"` // 是否从文件加载
}

type DBConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

type Config struct {
	Server  ServerConfig      `yaml:"server"`
	Redis   RedisConfig       `yaml:"redis"`
	DB      DBConfig          `yaml:"db"`
	Rate    RateConfig        `yaml:"rate"`
	JWT     JWTConfig         `yaml:"jwt"`
	AI      AIConfig          `yaml:"ai"`
	Log     zlog.LoggerConfig `yaml:"zlog"`
	QACache QACacheConfig     `yaml:"qa_cache"`
}

// LoadConfig 加载配置文件并返回 DBConfig
func LoadConfig() (*Config, error) {
	_, path, _, _ := runtime.Caller(0)
	configPath := filepath.Join(filepath.Dir(path), "config.yaml")
	var config Config
	if yamlFile, err := os.ReadFile(configPath); err == nil {
		if err := yaml.Unmarshal(yamlFile, &config); err != nil {
			return nil, fmt.Errorf("failed to parse config.yaml: %w", err)
		}
	} else {
		// config.yaml 不存在，使用零值（后续会被环境变量覆盖）
		zap.L().Info("config.yaml not found, using defaults from environment variables")
	}

	// 用环境变量覆盖所有字段（必须）
	ApplyEnvToConfig(&config)

	// 可选：验证必要字段是否已设置
	if config.AI.DashscopeAPIKey == "" {
		return nil, fmt.Errorf("required env DASHSCOPE_API_KEY is not set")
	}

	return &config, nil
}

// applyEnvToConfig 使用环境变量覆盖 config 中的字段（仅当环境变量非空时）
func ApplyEnvToConfig(cfg *Config) {
	getEnv := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}
	getEnvInt := func(key string, fallback int) int {
		if v := getEnv(key, ""); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
		return fallback
	}
	getEnvBool := func(key string, fallback bool) bool {
		if v := getEnv(key, ""); v != "" {
			if b, err := strconv.ParseBool(v); err == nil {
				return b
			}
		}
		return fallback
	}

	// Server
	cfg.Server.Port = getEnvInt("SERVER_PORT", cfg.Server.Port)

	// JWT
	cfg.JWT.Key = getEnv("JWT_KEY", cfg.JWT.Key)

	// Redis
	cfg.Redis.Host = getEnv("REDIS_HOST", cfg.Redis.Host)
	cfg.Redis.Port = getEnv("REDIS_PORT", cfg.Redis.Port)
	cfg.Redis.Password = getEnv("REDIS_PASSWORD", cfg.Redis.Password)
	cfg.Redis.DB = getEnvInt("REDIS_DB", cfg.Redis.DB)
	cfg.Redis.MaxConn = getEnvInt("REDIS_MAX_CONN", cfg.Redis.MaxConn)
	cfg.Redis.MaxIdleConn = getEnvInt("REDIS_MAX_IDLE_CONN", cfg.Redis.MaxIdleConn)

	// Rate
	cfg.Rate.UserRate = getEnvInt("RATE_USER_RATE", cfg.Rate.UserRate)
	cfg.Rate.UserBurst = getEnvInt("RATE_USER_BURST", cfg.Rate.UserBurst)

	// AI
	cfg.AI.DashscopeAPIKey = getEnv("DASHSCOPE_API_KEY", cfg.AI.DashscopeAPIKey)
	cfg.AI.DashscopeBaseURL = getEnv("DASHSCOPE_BASE_URL", cfg.AI.DashscopeBaseURL)
	cfg.AI.DashscopeModel = getEnv("DASHSCOPE_MODEL", cfg.AI.DashscopeModel)
	cfg.AI.SessionExpireSeconds = getEnvInt("SESSION_EXPIRE_SECONDS", cfg.AI.SessionExpireSeconds)
	cfg.AI.MaxSessionTurns = getEnvInt("MAX_SESSION_TURNS", cfg.AI.MaxSessionTurns)
	cfg.AI.MaxCtxToken = getEnvInt("MAX_CTX_TOKEN", cfg.AI.MaxCtxToken)

	// Log
	// 注意：zlog.Level 需要能从字符串解析
	if levelStr := getEnv("LOG_LEVEL", cfg.Log.Level.String()); levelStr != "" {
		cfg.Log.Level = zlog.Level(levelStr) // 假设 ParseLevel 存在且安全
	}
	cfg.Log.Output = getEnv("LOG_OUTPUT", cfg.Log.Output)
	cfg.Log.Format = getEnv("LOG_FORMAT", cfg.Log.Format)
	cfg.Log.FilePath = getEnv("LOG_FILE_PATH", cfg.Log.FilePath)
	cfg.Log.MaxSize = getEnvInt("LOG_MAX_SIZE", cfg.Log.MaxSize)
	cfg.Log.MaxBackups = getEnvInt("LOG_MAX_BACKUPS", cfg.Log.MaxBackups)
	cfg.Log.MaxAge = getEnvInt("LOG_MAX_AGE", cfg.Log.MaxAge)
	cfg.Log.Compress = getEnvBool("LOG_COMPRESS", cfg.Log.Compress)
	cfg.Log.Sampling = getEnvBool("LOG_SAMPLING", cfg.Log.Sampling)
	override := parseLogFieldsFromEnv()
	if override != nil {
		// 合并：保留 cfg.Log.Fields 已有字段，用 override 覆盖/新增
		if cfg.Log.Fields == nil {
			cfg.Log.Fields = make(map[string]string)
		}
		for k, v := range override {
			cfg.Log.Fields[k] = v
		}
	}

	// QACache
	cfg.QACache.Path = getEnv("QA_CACHE_DIR", cfg.QACache.Path)
	cfg.QACache.UseDB = getEnvBool("QA_CACHE_USE_DB", cfg.QACache.UseDB)
	cfg.QACache.LoadFromFile = getEnvBool("QA_CACHE_LOAD_FROM_FILE", cfg.QACache.LoadFromFile)
	
	// DB
	cfg.DB.Host = getEnv("DB_HOST", cfg.DB.Host)
	cfg.DB.Port = getEnv("DB_PORT", cfg.DB.Port)
	cfg.DB.User = getEnv("DB_USER", cfg.DB.User)
	cfg.DB.Password = getEnv("DB_PASSWORD", cfg.DB.Password)
	cfg.DB.DBName = getEnv("DB_NAME", cfg.DB.DBName)
	cfg.DB.SSLMode = getEnv("DB_SSLMODE", cfg.DB.SSLMode)
}

func parseLogFieldsFromEnv() map[string]string {
	raw := os.Getenv("LOG_FIELDS")
	if raw == "" {
		return nil // 或空 map
	}
	var fields map[string]string
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		log.Printf("Invalid LOG_FIELDS, ignoring: %v", err)
		return nil
	}
	return fields
}

func SyncConfigToEnv(cfg *Config) {
	setEnv := func(key, value string) { os.Setenv(key, value) }
	setEnvInt := func(key string, value int) { setEnv(key, strconv.Itoa(value)) }
	setEnvBool := func(key string, value bool) { setEnv(key, strconv.FormatBool(value)) }

	setEnvInt("SERVER_PORT", cfg.Server.Port)
	setEnv("JWT_KEY", cfg.JWT.Key)
	setEnv("REDIS_HOST", cfg.Redis.Host)
	setEnv("REDIS_PORT", cfg.Redis.Port)
	setEnv("REDIS_PASSWORD", cfg.Redis.Password)
	setEnvInt("REDIS_DB", cfg.Redis.DB)
	setEnvInt("REDIS_MAX_CONN", cfg.Redis.MaxConn)
	setEnvInt("REDIS_MAX_IDLE_CONN", cfg.Redis.MaxIdleConn)
	setEnvInt("RATE_USER_RATE", cfg.Rate.UserRate)
	setEnvInt("RATE_USER_BURST", cfg.Rate.UserBurst)
	setEnv("DASHSCOPE_API_KEY", cfg.AI.DashscopeAPIKey)
	setEnv("DASHSCOPE_BASE_URL", cfg.AI.DashscopeBaseURL)
	setEnv("DASHSCOPE_MODEL", cfg.AI.DashscopeModel)
	setEnvInt("SESSION_EXPIRE_SECONDS", cfg.AI.SessionExpireSeconds)
	setEnvInt("MAX_SESSION_TURNS", cfg.AI.MaxSessionTurns)
	setEnvInt("MAX_CTX_TOKEN", cfg.AI.MaxCtxToken)
	setEnv("LOG_LEVEL", cfg.Log.Level.String())
	setEnv("LOG_OUTPUT", cfg.Log.Output)
	setEnv("LOG_FORMAT", cfg.Log.Format)
	setEnv("LOG_FILE_PATH", cfg.Log.FilePath)
	setEnvInt("LOG_MAX_SIZE", cfg.Log.MaxSize)
	setEnvInt("LOG_MAX_BACKUPS", cfg.Log.MaxBackups)
	setEnvInt("LOG_MAX_AGE", cfg.Log.MaxAge)
	setEnvBool("LOG_COMPRESS", cfg.Log.Compress)
	setEnvBool("LOG_SAMPLING", cfg.Log.Sampling)
	setEnv("QA_CACHE_DIR", cfg.QACache.Path)
	setEnvBool("QA_CACHE_USE_DB", cfg.QACache.UseDB)
	setEnvBool("QA_CACHE_LOAD_FROM_FILE", cfg.QACache.LoadFromFile)
	
	setEnv("DB_HOST", cfg.DB.Host)
	setEnv("DB_PORT", cfg.DB.Port)
	setEnv("DB_USER", cfg.DB.User)
	setEnv("DB_PASSWORD", cfg.DB.Password)
	setEnv("DB_NAME", cfg.DB.DBName)
	setEnv("DB_SSLMODE", cfg.DB.SSLMode)
}
