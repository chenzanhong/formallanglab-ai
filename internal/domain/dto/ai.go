package dto

import (
	"github.com/chenzanhong/formallanglab-ai/internal/core"
	"github.com/chenzanhong/formallanglab-ai/internal/domain/model"
)

type AIChatRequest struct {
	Question  string           `json:"question" binding:"required"`
	Page      core.PageType    `json:"page" binding:"pageValid"`
	Automaton *model.Automaton `json:"automaton,omitempty"`
	Grammar   *model.Grammar   `json:"grammar,omitempty"`
	Regex     *model.Regex     `json:"regex,omitempty"`

	// 图片附件（多模态支持）
	Attachments []ImageAttachment `json:"attachments,omitempty"`
}

// ImageAttachment 图片附件
type ImageAttachment struct {
	Type     string `json:"type"`           // "image"
	MimeType string `json:"mimeType"`       // "image/jpeg", "image/png"
	Data     string `json:"data"`           // Base64 编码的图片数据
	Name     string `json:"name,omitempty"` // 文件名（可选）
}

// ========== Custom AI Model DTOs ===========
// CustomAIModelRequest 自定义 AI 模型请求
type CustomAIModelRequest struct {
	Name       string `json:"name" binding:"required"`
	Provider   string `json:"provider" binding:"required"`
	APIBaseURL string `json:"api_base_url" binding:"required"`
	APIKey     string `json:"api_key" binding:"required"`
	ModelName  string `json:"model_name" binding:"required"`
}

// CustomAIModelResponse 自定义 AI 模型响应
type CustomAIModelResponse struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	APIBaseURL string `json:"api_base_url"`
	ModelName  string `json:"model_name"`
	IsActive   bool   `json:"is_active"`
	CreatedAt  string `json:"created_at"`
}

// CustomAIModelListResponse 自定义 AI 模型列表响应
type CustomAIModelListResponse struct {
	Models []CustomAIModelResponse `json:"models"`
}

// CustomAIModelOperationResponse 自定义 AI 模型操作响应
type CustomAIModelOperationResponse struct {
	Result bool   `json:"result"`
	Msg    string `json:"msg"`
}

// SwitchModelRequest 切换模型请求
type SwitchModelRequest struct {
	ModelID int64 `json:"model_id"` // 0 表示切换到默认模型
}

// AIConfigResponse AI 配置响应
type AIConfigResponse struct {
	CurrentModelID int64                   `json:"current_model_id"` // 0 表示使用默认模型
	RemainingQuota int                     `json:"remaining_quota"`  // 默认模型剩余次数
	CustomModels   []CustomAIModelResponse `json:"custom_models"`    // 自定义模型列表
}
