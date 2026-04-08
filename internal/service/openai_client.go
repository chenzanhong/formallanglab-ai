package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chenzanhong/zlog"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

// generateCacheKey 生成缓存键
func generateCacheKey(apiKey, baseURL string) string {
	return fmt.Sprintf("%s|%s", baseURL, apiKey)
}

// cacheEntry 缓存条目，包含客户端和最后访问时间
type cacheEntry struct {
	client     *openai.Client
	lastAccess time.Time
}

// OpenAIClientManager 管理动态创建的 OpenAI 客户端
// 使用 TTL（Time-To-Live）策略清理长时间未使用的客户端
type OpenAIClientManager struct {
	clients map[string]*cacheEntry // 客户端缓存
	mu      sync.RWMutex           // 保护缓存访问
	ttl     time.Duration          // 过期时间
}

// NewOpenAIClientManager 创建一个新的客户端管理器
// ttl 指定客户端的过期时间，建议 30 分钟
func NewOpenAIClientManager(ttl time.Duration) *OpenAIClientManager {
	// 设置默认过期时间为 30 分钟
	if ttl <= 0 {
		ttl = 60 * time.Minute
	}

	m := &OpenAIClientManager{
		clients: make(map[string]*cacheEntry),
		ttl:     ttl,
	}

	// 启动后台清理协程，每 5 分钟清理一次过期客户端
	go m.cleanupLoop()

	return m
}

// GetClient 获取或创建 OpenAI 客户端
// 使用 apiKey 和 baseURL 的组合作为缓存键
func (m *OpenAIClientManager) GetClient(apiKey, baseURL string) (*openai.Client, error) {
	cacheKey := generateCacheKey(apiKey, baseURL)

	// 先尝试从缓存获取
	m.mu.RLock()
	if entry, exists := m.clients[cacheKey]; exists {
		// 检查是否过期
		if time.Since(entry.lastAccess) < m.ttl {
			m.mu.RUnlock()
			return entry.client, nil
		}
	}
	m.mu.RUnlock()

	// 缓存未命中或已过期，创建新客户端
	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查，防止并发创建
	if entry, exists := m.clients[cacheKey]; exists {
		if time.Since(entry.lastAccess) < m.ttl {
			return entry.client, nil
		}
	}

	// 创建新客户端
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)

	m.clients[cacheKey] = &cacheEntry{
		client:     &client,
		lastAccess: time.Now(),
	}

	return &client, nil
}

// GetDefaultClient 获取默认配置的客户端
func (m *OpenAIClientManager) GetDefaultClient(defaultAPIKey, defaultBaseURL string) (*openai.Client, error) {
	client, err := m.GetClient(defaultAPIKey, defaultBaseURL)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// ClearCache 清空客户端缓存
func (m *OpenAIClientManager) ClearCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients = make(map[string]*cacheEntry)
}

// RemoveClient 从缓存中移除特定客户端
func (m *OpenAIClientManager) RemoveClient(apiKey, baseURL string) {
	cacheKey := generateCacheKey(apiKey, baseURL)
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, cacheKey)
}

// cleanupLoop 定期清理过期客户端
func (m *OpenAIClientManager) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute) // 每 5 分钟清理一次
	defer ticker.Stop()

	for range ticker.C {
		m.cleanup()
	}
}

// cleanup 清理过期客户端
func (m *OpenAIClientManager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	count := 0
	for key, entry := range m.clients {
		if now.Sub(entry.lastAccess) > m.ttl {
			delete(m.clients, key)
			count++
		}
	}

	if count > 0 {
		zlog.Infow("清理过期客户端", "count", count)
	}
}

// validateClient 验证客户端配置是否有效（已废弃，请使用适配器的 ValidateClient 方法）
// 保留此函数仅用于兼容性，建议迁移到 ModelAdapter.ValidateClient
func validateClient(ctx context.Context, apiKey, baseURL string, modelName ...string) error {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)

	// 如果提供了模型名称，验证模型是否存在：尝试发起对话请求
	if len(modelName) > 0 && modelName[0] != "" {
		// 尝试获取指定模型的信息
		_, err := client.Models.Get(ctx, modelName[0])
		if err != nil {
			// 备选
			_, listErr := client.Models.List(ctx)
			if listErr != nil {
				return fmt.Errorf("模型 '%s' 验证失败：%w", modelName[0], listErr)
			}
			return nil
		}
		return nil
	}

	// 否则只验证 API Key 能否列出模型
	_, err := client.Models.List(ctx)
	if err != nil {
		return fmt.Errorf("OpenAI 客户端连接测试失败：%w", err)
	}

	return nil
}
