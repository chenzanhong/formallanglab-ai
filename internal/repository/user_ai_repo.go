package repository

import (
	"ai/internal/domain/model"
	"context"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// UserAIProfile 用户 AI 配置表，存储用户的 AI 相关配置
type UserAIProfile struct {
	ID             int64 `json:"id" gorm:"primarykey"`
	UserID         int64 `json:"user_id" gorm:"column:user_id"`                   // 关联 Auth 服务的用户 ID
	CurrentModelID int64 `json:"current_model_id" gorm:"column:current_model_id"` // 0 表示使用默认模型
}

// TableName 指定表名
func (UserAIProfile) TableName() string {
	return "user_ai_profiles"
}

// UserAIRepository 定义用户仓库接口
type UserAIRepository interface {
	// 自定义AI模型相关方法
	AddCustomAIModel(ctx context.Context, model *model.CustomAIModel) error
	GetCustomAIModels(ctx context.Context, userID int64) ([]*model.CustomAIModel, error)
	GetCustomAIModelByID(ctx context.Context, id int64, userID int64) (*model.CustomAIModel, error)
	UpdateCustomAIModel(ctx context.Context, model *model.CustomAIModel) error
	DeleteCustomAIModel(ctx context.Context, id int64, userID int64) error
	GetUserCurrentModelID(ctx context.Context, userID int64) (int64, error)
	UpdateUserCurrentModel(ctx context.Context, userID int64, modelID int64) error
	CheckModelOwnership(ctx context.Context, modelID int64, userID int64) (bool, error)
	EnsureUserAIProfile(ctx context.Context, userID int64) error
}

// UserAIRepositoryImpl 实现用户仓库接口
type UserAIRepositoryImpl struct {
	DB    *gorm.DB
	Redis *redis.Client
}

// NewUserAIRepository 创建用户仓库实例
func NewUserAIRepository(db *gorm.DB, redis *redis.Client) UserAIRepository {
	return &UserAIRepositoryImpl{DB: db, Redis: redis}
}

// AddCustomAIModel 添加自定义AI模型配置
func (r *UserAIRepositoryImpl) AddCustomAIModel(ctx context.Context, model *model.CustomAIModel) error {
	return r.DB.WithContext(ctx).Create(model).Error
}

// GetCustomAIModels 获取用户的自定义AI模型配置列表
func (r *UserAIRepositoryImpl) GetCustomAIModels(ctx context.Context, userID int64) ([]*model.CustomAIModel, error) {
	var models []*model.CustomAIModel
	if err := r.DB.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

// GetCustomAIModelByID 根据ID获取自定义AI模型配置
func (r *UserAIRepositoryImpl) GetCustomAIModelByID(ctx context.Context, id int64, userID int64) (*model.CustomAIModel, error) {
	var model model.CustomAIModel
	if err := r.DB.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// UpdateCustomAIModel 更新自定义AI模型配置
func (r *UserAIRepositoryImpl) UpdateCustomAIModel(ctx context.Context, model *model.CustomAIModel) error {
	return r.DB.WithContext(ctx).Save(model).Error
}

// DeleteCustomAIModel 删除自定义AI模型配置
func (r *UserAIRepositoryImpl) DeleteCustomAIModel(ctx context.Context, id int64, userID int64) error {
	// 开始事务
	tx := r.DB.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 检查该模型是否是用户当前使用的模型
	currentModelID, err := r.GetUserCurrentModelID(ctx, userID)
	if err != nil {
		tx.Rollback()
		return err
	}

	// 如果是当前使用的模型，将current_model_id设置为0（默认模型）
	if currentModelID != 0 && currentModelID == id {
		if err := r.UpdateUserCurrentModel(ctx, userID, 0); err != nil {
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
func (r *UserAIRepositoryImpl) GetUserCurrentModelID(ctx context.Context, userID int64) (int64, error) {
	var profile UserAIProfile
	result := r.DB.WithContext(ctx).Where("user_id = ?", userID).First(&profile)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// 用户AI配置不存在，返回0（默认模型）
			err := r.EnsureUserAIProfile(ctx, userID)
			if err != nil {
				return 0, err
			}
			return 0, nil
		}
		return 0, result.Error
	}
	return profile.CurrentModelID, nil
}

// UpdateUserCurrentModel 更新用户当前使用的模型ID
func (r *UserAIRepositoryImpl) UpdateUserCurrentModel(ctx context.Context, userID int64, modelID int64) error {
	// 确保用户AI配置存在
	err := r.EnsureUserAIProfile(ctx, userID)
	if err != nil {
		return err
	}

	return r.DB.WithContext(ctx).Model(&UserAIProfile{}).Where("user_id = ?", userID).Update("current_model_id", modelID).Error
}

// CheckModelOwnership 检查模型是否属于指定用户
func (r *UserAIRepositoryImpl) CheckModelOwnership(ctx context.Context, modelID int64, userID int64) (bool, error) {
	var count int64
	if err := r.DB.WithContext(ctx).Model(&model.CustomAIModel{}).Where("id = ? AND user_id = ?", modelID, userID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// EnsureUserAIProfile 确保用户AI配置记录存在
func (r *UserAIRepositoryImpl) EnsureUserAIProfile(ctx context.Context, userID int64) error {
	var count int64
	if err := r.DB.WithContext(ctx).Model(&UserAIProfile{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		profile := UserAIProfile{
			UserID:         userID,
			CurrentModelID: 0,
		}
		return r.DB.WithContext(ctx).Create(&profile).Error
	}

	return nil
}
