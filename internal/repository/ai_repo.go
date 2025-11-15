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
	GetSession(ctx context.Context, username, page string) (*model.AISession, error)
}

type AIRepositoryImpl struct {
	redis *redis.Client
}

func NewAIRepository(redisClient *redis.Client) AIRepository {
	return &AIRepositoryImpl{redis: redisClient}
}

func (r *AIRepositoryImpl) SaveSession(ctx context.Context, username string, session *model.AISession, sessionExpireSeconds int) error {
	key := "ai:" + username + ":" + string(session.Page)
	// fmt.Println(key)
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return r.redis.Set(ctx, key, data, time.Duration(sessionExpireSeconds)*time.Second).Err()
}

func (r *AIRepositoryImpl) GetSession(ctx context.Context, username, page string) (*model.AISession, error) {
	key := "ai:" + username + ":" + page
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
