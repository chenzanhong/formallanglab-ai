package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

// OpenAIClientManager 管理动态创建的OpenAI客户端
type OpenAIClientManager struct {
	clients map[string]*openai.Client
	mu      sync.RWMutex
}

// NewOpenAIClientManager 创建一个新的客户端管理器
func NewOpenAIClientManager() *OpenAIClientManager {
	return &OpenAIClientManager{
		clients: make(map[string]*openai.Client),
	}
}

// GetClient 获取或创建OpenAI客户端
// 使用apiKey和baseURL的组合作为缓存键
func (m *OpenAIClientManager) GetClient(apiKey, baseURL string) *openai.Client {
	// 生成缓存键
	cacheKey := fmt.Sprintf("%s|%s", baseURL, apiKey)

	// 先尝试从缓存获取
	m.mu.RLock()
	if client, exists := m.clients[cacheKey]; exists {
		m.mu.RUnlock()
		return client
	}
	m.mu.RUnlock()

	// 缓存未命中，创建新客户端
	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查，防止并发创建
	if client, exists := m.clients[cacheKey]; exists {
		return client
	}

	// 创建新客户端
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)

	m.clients[cacheKey] = &client
	return &client
}

// GetDefaultClient 获取默认配置的客户端
func (m *OpenAIClientManager) GetDefaultClient(defaultAPIKey, defaultBaseURL string) *openai.Client {
	return m.GetClient(defaultAPIKey, defaultBaseURL)
}

// ClearCache 清空客户端缓存
func (m *OpenAIClientManager) ClearCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients = make(map[string]*openai.Client)
}

// RemoveClient 从缓存中移除特定客户端
func (m *OpenAIClientManager) RemoveClient(apiKey, baseURL string) {
	cacheKey := fmt.Sprintf("%s|%s", baseURL, apiKey)
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, cacheKey)
}

// ValidateClient 验证客户端配置是否有效
func ValidateClient(ctx context.Context, apiKey, baseURL string) error {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)

	_, err := client.Models.List(ctx)
	if err != nil {
		return fmt.Errorf("OpenAI客户端连接测试失败: %v", err)
	}

	return nil
}
