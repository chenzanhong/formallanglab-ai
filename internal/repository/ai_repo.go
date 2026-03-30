package repository

import (
	"ai/internal/domain/model"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type AIRepository interface {
	SaveSession(ctx context.Context, username string, session *model.AISession, sessionExpireSeconds int) error
	GetSession(ctx context.Context, username string) (*model.AISession, error)
	GetAICallCount(ctx context.Context, userID string) (int, error)
	IncrementAICallCount(ctx context.Context, userID string) error
	ResetAICallCount(ctx context.Context, userID string) error
	GetCustomAIModelFromRedis(ctx context.Context, modelID int64, userID int64) (map[string]string, error)
	SaveCustomAIModelToRedis(ctx context.Context, modelID int64, userID int64, modelConfig map[string]string) error
	DeleteCustomAIModelFromRedis(ctx context.Context, modelID int64, userID int64) error
}

type AIRepositoryImpl struct {
	redis *redis.Client
}

func NewAIRepository(redisClient *redis.Client) AIRepository {
	return &AIRepositoryImpl{redis: redisClient}
}

func (r *AIRepositoryImpl) SaveSession(ctx context.Context, username string, session *model.AISession, sessionExpireSeconds int) error {
	key := "ai:" + username
	// fmt.Println(key)
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return r.redis.Set(ctx, key, data, time.Duration(sessionExpireSeconds)*time.Second).Err()
}

func (r *AIRepositoryImpl) GetSession(ctx context.Context, username string) (*model.AISession, error) {
	key := "ai:" + username
	data, err := r.redis.Get(ctx, key).Bytes()
	if err != nil {
		// 判断是否是 Redis 的 "key not found" 错误
		if errors.Is(err, redis.Nil) {
			// key 不存在，视为“无会话”，不返回错误
			return nil, nil
		}
		// 其他错误（如连接失败、超时等）需要返回
		return nil, fmt.Errorf("failed to get session from redis: %w", err)
	}
	var session model.AISession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// GetAICallCount 获取用户AI调用次数
func (r *AIRepositoryImpl) GetAICallCount(ctx context.Context, userID string) (int, error) {
	date := time.Now().Format("20060102")
	key := "ai-" + date + "-" + userID + "-num"
	count, err := r.redis.Get(ctx, key).Int()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// 键不存在，返回0
			return 0, nil
		}
		return 0, err
	}
	return count, nil
}

// IncrementAICallCount 增加用户AI调用次数
func (r *AIRepositoryImpl) IncrementAICallCount(ctx context.Context, userID string) error {
	date := time.Now().Format("20060102")
	key := "ai-" + date + "-" + userID + "-num"
	// 增加计数，设置24小时过期
	err := r.redis.Incr(ctx, key).Err()
	if err != nil {
		return err
	}
	// 设置过期时间（如果是新键）
	r.redis.Expire(ctx, key, 24*time.Hour)
	return nil
}

// ResetAICallCount 重置用户AI调用次数
func (r *AIRepositoryImpl) ResetAICallCount(ctx context.Context, userID string) error {
	date := time.Now().Format("20060102")
	key := "ai-" + date + "-" + userID + "-num"
	return r.redis.Del(ctx, key).Err()
}

// GetCustomAIModelFromRedis 从Redis获取自定义AI模型配置
func (r *AIRepositoryImpl) GetCustomAIModelFromRedis(ctx context.Context, modelID int64, userID int64) (map[string]string, error) {
	key := fmt.Sprintf("ai-model:%d:%d", userID, modelID)
	data, err := r.redis.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get ai model from redis: %w", err)
	}

	var modelConfig map[string]string
	if err := json.Unmarshal(data, &modelConfig); err != nil {
		return nil, err
	}
	return modelConfig, nil
}

// SaveCustomAIModelToRedis 保存自定义AI模型配置到Redis
func (r *AIRepositoryImpl) SaveCustomAIModelToRedis(ctx context.Context, modelID int64, userID int64, modelConfig map[string]string) error {
	key := fmt.Sprintf("ai-model:%d:%d", userID, modelID)
	data, err := json.Marshal(modelConfig)
	if err != nil {
		return err
	}
	return r.redis.Set(ctx, key, data, 24*time.Hour).Err()
}

// DeleteCustomAIModelFromRedis 从Redis删除自定义AI模型配置
func (r *AIRepositoryImpl) DeleteCustomAIModelFromRedis(ctx context.Context, modelID int64, userID int64) error {
	key := fmt.Sprintf("ai-model:%d:%d", userID, modelID)
	return r.redis.Del(ctx, key).Err()
}
