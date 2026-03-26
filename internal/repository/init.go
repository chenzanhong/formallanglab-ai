package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/chenzanhong/zlog"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func InitRedis() (*redis.Client, error) {
	host := os.Getenv("REDIS_HOST")
	port := os.Getenv("REDIS_PORT")
	password := os.Getenv("REDIS_PASSWORD")
	dbStr := os.Getenv("REDIS_DB")

	db := 0
	if dbStr != "" {
		var err error
		db, err = strconv.Atoi(dbStr)
		if err != nil {
			return nil, fmt.Errorf("invalid REDIS_DB value: %v", err)
		}
	}

	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", host, port),
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		MaxRetries:   5, // default 3

		// 关键：启用连接池健康检查
		PoolSize:        20,
		MinIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 10 * time.Minute,
	})

	// 测试连接
	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	zlog.Info("Redis connected successfully")
	return client, nil
}

func InitDB() (*gorm.DB, error) {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	ctx := context.Background()
	if err := InitPGData(db, ctx); err != nil {
		return nil, fmt.Errorf("failed to init pg data: %w", err)
	}

	zlog.Info("Database connected successfully")
	return db, nil
}

// InitPGData 执行 PostgreSQL 数据初始化（执行 migrations 目录下的 SQL 文件）
func InitPGData(db *gorm.DB, ctx context.Context) error {
	if db == nil {
		return fmt.Errorf("database connection not initialized")
	}

	migrationsDir := "./migrations" // 相对于工作目录
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	tx := db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".sql" {
			filePath := filepath.Join(migrationsDir, file.Name())
			fmt.Println("Execute: ", filePath)
			content, err := os.ReadFile(filePath)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("读取文件失败 %s: %w", filePath, err)
			}

			if err = tx.WithContext(ctx).Exec(string(content)).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("执行 SQL 失败 %s: %w", filePath, err)
			}
		}
	}

	if err := tx.WithContext(ctx).Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	fmt.Println("Migration completed successfully.")

	return nil
}
