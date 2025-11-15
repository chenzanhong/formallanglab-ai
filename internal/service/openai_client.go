package service

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

// NewOpenAIClient 创建OpenAI客户端
func NewOpenAIClient(apiKey, baseURL string) (*openai.Client, error) {

	// 根据配置创建客户端
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)

	// 测试连接（可选）
	_, err := client.Models.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("OpenAI客户端连接测试失败: %v, 继续启动但AI功能可能不可用", err)
	}

	return &client, nil
}
