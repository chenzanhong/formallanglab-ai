package repository

import (
	"ai/internal/domain/model"
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/chenzanhong/zlog"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID             int64                 `json:"id" gorm:"primarykey"`
	Name           string                `json:"name"`
	Password       string                `json:"password"`
	Token          string                `json:"token"`
	Email          string                `json:"email"`
	CurrentModelID *int64                `json:"current_model_id" gorm:"default:null"` // 为nil表示使用默认模型
	CustomAIModels []model.CustomAIModel `json:"custom_ai_models" gorm:"foreignKey:UserID"`
}

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

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, host, port, dbname)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// 自动迁移模型
	err = db.AutoMigrate(&User{}, &model.CustomAIModel{})
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %v", err)
	}

	zlog.Info("Database connected successfully")
	return db, nil
}

// UserRepository 定义用户仓库接口
type UserRepository interface {
	// 自定义AI模型相关方法
	AddCustomAIModel(ctx context.Context, model *model.CustomAIModel) error
	GetCustomAIModels(ctx context.Context, userID int64) ([]*model.CustomAIModel, error)
	GetCustomAIModelByID(ctx context.Context, id int64, userID int64) (*model.CustomAIModel, error)
	UpdateCustomAIModel(ctx context.Context, model *model.CustomAIModel) error
	DeleteCustomAIModel(ctx context.Context, id int64, userID int64) error
	GetUserCurrentModelID(ctx context.Context, userID int64) (*int64, error)
	UpdateUserCurrentModel(ctx context.Context, userID int64, modelID *int64) error
	CheckModelOwnership(ctx context.Context, modelID int64, userID int64) (bool, error)
}

// UserRepositoryImpl 实现用户仓库接口
type UserRepositoryImpl struct {
	DB    *gorm.DB
	Redis *redis.Client
}

// NewUserRepository 创建用户仓库实例
func NewUserRepository(db *gorm.DB, redis *redis.Client) UserRepository {
	return &UserRepositoryImpl{DB: db, Redis: redis}
}

// AddCustomAIModel 添加自定义AI模型配置
func (r *UserRepositoryImpl) AddCustomAIModel(ctx context.Context, model *model.CustomAIModel) error {
	return r.DB.WithContext(ctx).Create(model).Error
}

// GetCustomAIModels 获取用户的自定义AI模型配置列表
func (r *UserRepositoryImpl) GetCustomAIModels(ctx context.Context, userID int64) ([]*model.CustomAIModel, error) {
	var models []*model.CustomAIModel
	if err := r.DB.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

// GetCustomAIModelByID 根据ID获取自定义AI模型配置
func (r *UserRepositoryImpl) GetCustomAIModelByID(ctx context.Context, id int64, userID int64) (*model.CustomAIModel, error) {
	var model model.CustomAIModel
	if err := r.DB.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// UpdateCustomAIModel 更新自定义AI模型配置
func (r *UserRepositoryImpl) UpdateCustomAIModel(ctx context.Context, model *model.CustomAIModel) error {
	return r.DB.WithContext(ctx).Save(model).Error
}

// DeleteCustomAIModel 删除自定义AI模型配置
func (r *UserRepositoryImpl) DeleteCustomAIModel(ctx context.Context, id int64, userID int64) error {
	// 开始事务
	tx := r.DB.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 检查该模型是否是用户当前使用的模型
	var user User
	if err := tx.Select("current_model_id").Where("id = ?", userID).First(&user).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 如果是当前使用的模型，将current_model_id设置为nil
	if user.CurrentModelID != nil && *user.CurrentModelID == id {
		if err := tx.Model(&User{}).Where("id = ?", userID).Update("current_model_id", nil).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// 删除模型
	if err := tx.Where("id = ? AND user_id = ?", id, userID).Delete(&model.CustomAIModel{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// GetUserCurrentModelID 获取用户当前使用的模型ID
func (r *UserRepositoryImpl) GetUserCurrentModelID(ctx context.Context, userID int64) (*int64, error) {
	var user User
	result := r.DB.WithContext(ctx).Select("current_model_id").Where("id = ?", userID).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil // 用户不存在，返回nil
		}
		return nil, result.Error
	}
	return user.CurrentModelID, nil
}

// UpdateUserCurrentModel 更新用户当前使用的模型ID
func (r *UserRepositoryImpl) UpdateUserCurrentModel(ctx context.Context, userID int64, modelID *int64) error {
	return r.DB.WithContext(ctx).Model(&User{}).Where("id = ?", userID).Update("current_model_id", modelID).Error
}

// CheckModelOwnership 检查模型是否属于指定用户
func (r *UserRepositoryImpl) CheckModelOwnership(ctx context.Context, modelID int64, userID int64) (bool, error) {
	var count int64
	if err := r.DB.WithContext(ctx).Model(&model.CustomAIModel{}).Where("id = ? AND user_id = ?", modelID, userID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
