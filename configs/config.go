package configs

import (
	// "ai/internal/middleware"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

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
	Path string `yaml:"path"`
}

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Redis   RedisConfig   `yaml:"redis"`
	Rate    RateConfig    `yaml:"rate"`
	JWT     JWTConfig     `yaml:"jwt"`
	AI      AIConfig      `yaml:"ai"`
	Log     LogConfig     `yaml:"log"`
	QACache QACacheConfig `yaml:"qa_cache"`
}

// getConfigPath 获取数据库配置文件的路径
func getConfigPath() string {
	_, filename, _, ok := runtime.Caller(2) // 获取调用者的文件名
	if !ok {
		log.Fatal("无法获取运行时调用者信息")
	}

	// 获取当前文件所在的目录
	currentDir := filepath.Dir(filename)

	// 构建到项目根目录的相对路径
	dbConfigPath := filepath.Join(currentDir, "..", "configs", "config.yaml")

	// 将路径转换为绝对路径并简化路径
	absPath, err := filepath.Abs(dbConfigPath)
	if err != nil {
		log.Printf("无法获取绝对路径: %v", err)
	}

	simplifiedPath := filepath.Clean(absPath)

	return simplifiedPath
}

// GetConfigPath 返回数据库配置文件的路径
func GetConfigPath() string {
	return getConfigPath()
}

// LoadConfig 加载配置文件并返回 DBConfig
func LoadConfig() (*Config, error) {
	configPath := GetConfigPath()
	yamlFile, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func SetEnvVariables(config *Config) {
	// 辅助函数：如果 envVar 未设置，则用 fallback 值设置它
	setEnvIfNotSet := func(envVar, fallback string) {
		if os.Getenv(envVar) == "" {
			os.Setenv(envVar, fallback)
		}
	}
	// Server
	setEnvIfNotSet("SERVER_PORT", strconv.Itoa(config.Server.Port))

	// JWT
	setEnvIfNotSet("JWT_KEY", config.JWT.Key)
	// middleware.SetJWTKey(os.Getenv("JWT_KEY"))

	// Redis
	setEnvIfNotSet("REDIS_HOST", config.Redis.Host)
	setEnvIfNotSet("REDIS_PORT", config.Redis.Port)
	setEnvIfNotSet("REDIS_PASSWORD", config.Redis.Password)
	setEnvIfNotSet("REDIS_DB", strconv.Itoa(config.Redis.DB))
	setEnvIfNotSet("REDIS_MAX_CONN", strconv.Itoa(config.Redis.MaxConn))
	setEnvIfNotSet("REDIS_MAX_IDLE_CONN", strconv.Itoa(config.Redis.MaxIdleConn))

	// Rate Limiting
	setEnvIfNotSet("RATE_USER_RATE", strconv.Itoa(config.Rate.UserRate))
	setEnvIfNotSet("RATE_USER_BURST", strconv.Itoa(config.Rate.UserBurst))

	// AI Service
	setEnvIfNotSet("DASHSCOPE_API_KEY", config.AI.DashscopeAPIKey)
	setEnvIfNotSet("DASHSCOPE_BASE_URL", config.AI.DashscopeBaseURL)
	setEnvIfNotSet("DASHSCOPE_MODEL", config.AI.DashscopeModel)
	setEnvIfNotSet("SESSION_EXPIRE_SECONDS", strconv.Itoa(config.AI.SessionExpireSeconds))
	setEnvIfNotSet("MAX_SESSION_TURNS", strconv.Itoa(config.AI.MaxSessionTurns))
	// setEnvIfNotSet("CHROMA_URL", config.AI.ChromaURL)
	// setEnvIfNotSet("CHROMA_COLLECTION", config.AI.ChromaCollection)

	// Log
	setEnvIfNotSet("LOG_LEVEL", config.Log.Level)
	setEnvIfNotSet("LOG_OUTPUT", config.Log.Output)
	setEnvIfNotSet("LOG_FORMAT", config.Log.Format)
	setEnvIfNotSet("LOG_FILE_PATH", config.Log.FilePath)
	setEnvIfNotSet("LOG_MAX_SIZE", strconv.Itoa(config.Log.MaxSize))
	setEnvIfNotSet("LOG_MAX_BACKUPS", strconv.Itoa(config.Log.MaxBackups))
	setEnvIfNotSet("LOG_MAX_AGE", strconv.Itoa(config.Log.MaxAge))
	setEnvIfNotSet("LOG_COMPRESS", strconv.FormatBool(config.Log.Compress))
	setEnvIfNotSet("LOG_SAMPLING", strconv.FormatBool(config.Log.Sampling))

	// QACache
	setEnvIfNotSet("QA_CACHE_DIR", config.QACache.Path)
}
