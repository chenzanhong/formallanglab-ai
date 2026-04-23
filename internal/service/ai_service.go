package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/chenzanhong/zlog"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/packages/ssestream"
	"gorm.io/gorm"

	"github.com/chenzanhong/formallanglab-ai/configs"
	"github.com/chenzanhong/formallanglab-ai/internal/domain/dto"
	"github.com/chenzanhong/formallanglab-ai/internal/domain/model"
	"github.com/chenzanhong/formallanglab-ai/internal/repository"
	"github.com/chenzanhong/formallanglab-ai/internal/utils"
)

const (
	systemPrompt = `你是一位形式语言与自动机理论课程的教学助教，专注于自动机、正则表达式、文法和图灵机等内容。
	
目前和学生一对一交流，回答中不要用“你们”，而应该用“你”。

请用清晰、准确、循序渐进的方式回答问题，优先解释原理和“为什么”，必要时举例说明。

如果学生提供了文法、自动机或正则表达式等结构，请结合具体情境（是否需要结合提供的结构进行回答）以及已有对话内容进行分析。

不要虚构定理或算法，不确定时请说明“标准理论中通常……”。

注意！对于明显超出形式语言与自动机范围的问题（如编程、其他课程内容等），请礼貌回应，例如：
> “抱歉，这个问题超出了本课程《形式语言与自动机》的范围，我无法回答该问题。”
`
	maxAICallPerDay = 10
)

type AIService interface {
	StreamChat(ctx context.Context, session *model.AISession, req *dto.AIChatRequest, modelID int64, userID int64) (*ssestream.Stream[openai.ChatCompletionChunk], error)
	MockStreamChat(ctx context.Context, cacheAnswer string) (*model.MockStream, error)
	CheckCache(question string) string                                         // 查看是否命中预置高频问题缓存
	GetSession(ctx context.Context, username string) (*model.AISession, error) //
	SaveSession(ctx context.Context, username string, session *model.AISession) error
	ClearSession(ctx context.Context, username string) error
	GetSessionExpireSeconds() int
	GetMaxSessionTurns() int
	CheckAICallLimit(ctx context.Context, userID int64) (bool, error) // 检查是否超出调用限制
	IncrementAICallCount(ctx context.Context, userID int64) error     // 增加调用计数

	GetCustomAIModel(ctx context.Context, modelID int64, userID int64) (map[string]string, error)
	GetCustomAIModels(ctx context.Context, userID int64) ([]*model.CustomAIModel, error)
	ValidateClient(ctx context.Context, apiKey, baseURL, modelName, provider string) error
	AddCustomAIModel(ctx context.Context, userID int64, req *dto.CustomAIModelRequest) (*model.CustomAIModel, error)
	UpdateCustomAIModel(ctx context.Context, userID int64, modelID int64, req *dto.CustomAIModelRequest) (*model.CustomAIModel, error)
	DeleteCustomAIModel(ctx context.Context, userID int64, modelID int64) error
	GetUserCurrentModelID(ctx context.Context, userID int64) (int64, error)
	UpdateUserCurrentModel(ctx context.Context, userID int64, modelID int64) error
	GetAIConfig(ctx context.Context, userID int64) (*dto.AIConfigResponse, error)
}

type AIServiceImpl struct {
	clientManager *OpenAIClientManager
	repo          repository.AIRepository
	qaCache       *model.QACache
	aiCfg         configs.AIConfig
	userAIRepo    repository.UserAIRepository
}

func NewAIService(
	clientManager *OpenAIClientManager,
	repo repository.AIRepository,
	userAIRepo repository.UserAIRepository,
	qaCache *model.QACache,
	aiCfg configs.AIConfig,
) AIService {
	return &AIServiceImpl{
		clientManager: clientManager,
		repo:          repo,
		userAIRepo:    userAIRepo,
		qaCache:       qaCache,
		aiCfg:         aiCfg,
	}
}

// StreamChat 处理流式对话请求
func (s *AIServiceImpl) StreamChat(ctx context.Context, session *model.AISession, req *dto.AIChatRequest, modelID int64, userID int64) (*ssestream.Stream[openai.ChatCompletionChunk], error) {
	// 构造消息：严格遵循 [system] → [history] → [context] → [current question]
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
	}

	// 历史对话：直接交替添加
	for _, turn := range session.RecentTurns {
		messages = append(messages, openai.UserMessage(turn.User))
		messages = append(messages, openai.AssistantMessage(turn.AI))
	}

	// 当前页面上下文（如自动机/文法/正则）
	if ctxPrompt := s.buildContextualPrompt(req); ctxPrompt != "" {
		messages = append(messages, openai.UserMessage(ctxPrompt))
	}

	// 用户当前问题
	messages = append(messages, openai.UserMessage(req.Question))

	// 调用流式 API
	var stream *ssestream.Stream[openai.ChatCompletionChunk]

	if modelID > 0 {
		// 使用自定义模型
		modelConfig, err := s.GetCustomAIModel(ctx, modelID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get custom model: %w", err)
		}

		// 解密 API 密钥
		decryptedAPIKey, err := utils.Decrypt(modelConfig["api_key"], s.aiCfg.CryptoKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt API key: %w", err)
		}
		baseURL := modelConfig["base_url"]

		// 动态创建客户端
		client, err := s.clientManager.GetClient(decryptedAPIKey, baseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to get client: %w", err)
		}
		stream = client.Chat.Completions.NewStreaming(
			ctx, openai.ChatCompletionNewParams{
				Messages: messages,
				Model:    modelConfig["model"],
			},
		)

		return stream, nil
	}

	// 使用默认模型
	client, err := s.clientManager.GetDefaultClient(s.aiCfg.DashscopeAPIKey, s.aiCfg.DashscopeBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get default client: %w", err)
	}
	stream = client.Chat.Completions.NewStreaming(
		ctx, openai.ChatCompletionNewParams{
			Messages: messages,
			Model:    os.Getenv("DASHSCOPE_MODEL"),
		},
	)

	return stream, nil
}

func (s *AIServiceImpl) buildMessagesWithTruncation(
	session *model.AISession,
	req *dto.AIChatRequest,
) []openai.ChatCompletionMessageParamUnion {
	// Step 1: 构造必须保留的消息（L1）
	systemMsg := openai.SystemMessage(systemPrompt)
	currentQuestion := openai.UserMessage(req.Question)
	var ctxMsg openai.ChatCompletionMessageParamUnion
	ctxPrompt := s.buildContextualPrompt(req)
	if ctxPrompt != "" {
		ctxMsg = openai.UserMessage(ctxPrompt)
	}

	// 计算 L1 总 token
	l1Messages := []openai.ChatCompletionMessageParamUnion{systemMsg, currentQuestion}
	if ctxPrompt != "" {
		l1Messages = append(l1Messages, ctxMsg)
	}
	l1Tokens := s.estimateMessagesTokens(l1Messages)
	if l1Tokens >= s.aiCfg.MaxCtxToken {
		// 极端情况：连 L1 都超了 → 强制只保留 system + current question
		return []openai.ChatCompletionMessageParamUnion{systemMsg, currentQuestion}
	}

	// Step 2: 尝试从 RecentTurns 尾部向前添加（L2），直到快满
	availableTokens := s.aiCfg.MaxCtxToken - l1Tokens
	historyMessages := []openai.ChatCompletionMessageParamUnion{}

	// 从最新轮次开始倒序遍历（确保最新对话优先保留）
	for i := len(session.RecentTurns) - 1; i >= 0; i-- {
		turn := session.RecentTurns[i]

		candidate := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(turn.User),
			openai.AssistantMessage(turn.AI),
		}
		candidateTokens := s.estimateMessagesTokens(candidate)

		if candidateTokens <= availableTokens {
			// 插入到 historyMessages 开头（保持时间顺序）
			historyMessages = append(candidate, historyMessages...)
			availableTokens -= candidateTokens
		} else {
			// 当前轮次放不下，后续更早的也不用看了
			break
		}
	}

	// Step 3: 拼接最终消息列表（system → history → context → current）
	finalMessages := []openai.ChatCompletionMessageParamUnion{systemMsg}
	finalMessages = append(finalMessages, historyMessages...)
	if ctxPrompt != "" {
		finalMessages = append(finalMessages, ctxMsg)
	}
	finalMessages = append(finalMessages, currentQuestion)

	return finalMessages
}

// 辅助函数：估算一组消息的总 token 数
func (s *AIServiceImpl) estimateMessagesTokens(messages []openai.ChatCompletionMessageParamUnion) int {
	total := 0

	// 加上角色标签等开销（每条消息约 +5~10 tokens）
	total += len(messages) * 8

	return total
}

// MockStreamChat 模拟流式响应，用于响应预置高频问题缓存
func (s *AIServiceImpl) MockStreamChat(ctx context.Context, cacheAnswer string) (*model.MockStream, error) {
	return model.NewMockStream(cacheAnswer), nil
}

// CheckCache 查看是否命中预置高频问题缓存
func (s *AIServiceImpl) CheckCache(question string) string {
	return s.qaCache.Get(question)
}

// SaveSession 保存会话
func (s *AIServiceImpl) SaveSession(ctx context.Context, username string, session *model.AISession) error {
	return s.repo.SaveSession(ctx, username, session, s.aiCfg.SessionExpireSeconds)
}

// ClearSession 清除会话
func (s *AIServiceImpl) ClearSession(ctx context.Context, username string) error {
	return s.repo.DeleteSession(ctx, username)
}

func (s *AIServiceImpl) GetSession(ctx context.Context, username string) (*model.AISession, error) {
	session, err := s.repo.GetSession(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}
	if session == nil {
		session = &model.AISession{}
	}

	return session, nil
}

// GetSessionExpireSeconds 返回会话过期时间（秒）
func (s *AIServiceImpl) GetSessionExpireSeconds() int {
	return s.aiCfg.SessionExpireSeconds
}

// GetMaxSessionTurns 返回最大会话轮数
func (s *AIServiceImpl) GetMaxSessionTurns() int {
	return s.aiCfg.MaxSessionTurns
}

// CheckAICallLimit 检查用户是否超出 AI 调用限制
func (s *AIServiceImpl) CheckAICallLimit(ctx context.Context, userID int64) (bool, error) {
	count, err := s.repo.GetAICallCount(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get AI call count: %w", err)
	}

	return count < maxAICallPerDay, nil
}

// IncrementAICallCount 增加用户 AI 调用计数
func (s *AIServiceImpl) IncrementAICallCount(ctx context.Context, userID int64) error {
	return s.repo.IncrementAICallCount(ctx, userID)
}

// GetCustomAIModel 获取自定义 AI 模型配置
func (s *AIServiceImpl) GetCustomAIModel(ctx context.Context, modelID int64, userID int64) (map[string]string, error) {
	modelConfig, err := s.repo.GetCustomAIModelFromRedis(ctx, modelID, userID)
	if err != nil {
		zlog.Error("failed to get custom ai model from redis", zlog.String("err", err.Error()))
	}

	if modelConfig != nil {
		return modelConfig, nil
	}

	model, err := s.userAIRepo.GetCustomAIModelByID(ctx, modelID, userID)
	if err != nil {
		return nil, err
	}

	// 保持 API key 加密状态，直接存储到 Redis
	modelConfig = map[string]string{
		"api_key":  model.APIKey,
		"base_url": model.APIBaseURL,
		"model":    model.ModelName,
		"provider": model.Provider,
	}

	if err := s.repo.SaveCustomAIModelToRedis(ctx, modelID, userID, modelConfig); err != nil {
		return nil, err
	}

	return modelConfig, nil
}

// ValidateModelConfig 验证模型配置是否非空
func ValidateModelConfig(provider, apiKey, baseURL, modelName string) error {
	if provider == "" {
		return fmt.Errorf("服务商不能为空")
	}

	if apiKey == "" {
		return fmt.Errorf("API Key 不能为空")
	}

	if modelName == "" {
		return fmt.Errorf("模型名称不能为空")
	}

	// 验证 Base URL
	if baseURL == "" {
		return fmt.Errorf("Base URL 不能为空")
	}

	return nil
}

// AddCustomAIModel 添加自定义 AI 模型配置
func (s *AIServiceImpl) AddCustomAIModel(ctx context.Context, userID int64, req *dto.CustomAIModelRequest) (*model.CustomAIModel, error) {
	// 验证模型配置
	if err := ValidateModelConfig(req.Provider, req.APIKey, req.APIBaseURL, req.ModelName); err != nil {
		return nil, err
	}

	// 加密 API key
	encryptedAPIKey, err := utils.Encrypt(req.APIKey, s.aiCfg.CryptoKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt API key: %w", err)
	}

	aiModel := &model.CustomAIModel{
		UserID:     userID,
		Name:       req.Name,
		Provider:   req.Provider,
		APIKey:     encryptedAPIKey,
		APIBaseURL: req.APIBaseURL,
		ModelName:  req.ModelName,
		IsActive:   true,
	}

	if err := s.userAIRepo.AddCustomAIModel(ctx, aiModel); err != nil {
		// 是否是唯一键冲突（API key 已存在）
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, fmt.Errorf("the configuration already exists")
		}

		return nil, err
	}

	modelConfig := map[string]string{
		"api_key":  encryptedAPIKey,
		"base_url": aiModel.APIBaseURL,
		"model":    aiModel.ModelName,
		"provider": aiModel.Provider,
	}

	if err := s.repo.SaveCustomAIModelToRedis(ctx, aiModel.ID, userID, modelConfig); err != nil {
		return nil, fmt.Errorf("failed to save custom ai model to redis: %w", err)
	}

	return aiModel, nil
}

// GetCustomAIModels 获取用户的自定义 AI 模型配置列表
func (s *AIServiceImpl) GetCustomAIModels(ctx context.Context, userID int64) ([]*model.CustomAIModel, error) {
	models, err := s.userAIRepo.GetCustomAIModels(ctx, userID)
	if err != nil {
		return nil, err
	}

	return models, nil
}

// ValidateClient 验证自定义 AI 模型配置是否有效
func (s *AIServiceImpl) ValidateClient(ctx context.Context, apiKey, baseURL, modelName, provider string) error {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)

	// 如果有模型名称，直接发起一次流式对话测试
	if modelName != "" {
		stream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
			Model: modelName,
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("你好，请回复 1"),
			},
			MaxTokens: openai.Int(1),
		})
		defer stream.Close()

		// 读取第一个响应块，验证连接和模型是否可用
		if stream.Next() {
			// 成功接收到响应
			return nil
		}

		// 检查是否有错误
		if err := stream.Err(); err != nil {
			return fmt.Errorf("模型 '%s' 流式对话测试失败：%w", modelName, err)
		}

		return fmt.Errorf("模型 '%s' 未返回任何响应", modelName)
	}

	// 没有模型名称，只验证 API Key 能否列出模型
	_, err := client.Models.List(ctx)
	if err != nil {
		return fmt.Errorf("大模型 API 连接测试失败：%w", err)
	}

	return nil
}

// UpdateCustomAIModel 更新自定义 AI 模型配置
func (s *AIServiceImpl) UpdateCustomAIModel(ctx context.Context, userID int64, modelID int64, req *dto.CustomAIModelRequest) (*model.CustomAIModel, error) {
	// 验证模型配置
	if err := ValidateModelConfig(req.Provider, req.APIKey, req.APIBaseURL, req.ModelName); err != nil {
		return nil, err
	}

	existingModel, err := s.userAIRepo.GetCustomAIModelByID(ctx, modelID, userID)
	if err != nil {
		return nil, err
	}

	// 加密 API key
	encryptedAPIKey, err := utils.Encrypt(req.APIKey, s.aiCfg.CryptoKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt API key: %w", err)
	}

	existingModel.Name = req.Name
	existingModel.Provider = req.Provider
	existingModel.APIKey = encryptedAPIKey
	existingModel.APIBaseURL = req.APIBaseURL
	existingModel.ModelName = req.ModelName
	existingModel.IsActive = true

	if err := s.userAIRepo.UpdateCustomAIModel(ctx, existingModel); err != nil {
		return nil, err
	}

	modelConfig := map[string]string{
		"api_key":  encryptedAPIKey,
		"base_url": existingModel.APIBaseURL,
		"model":    existingModel.ModelName,
		"provider": existingModel.Provider,
	}

	if err := s.repo.SaveCustomAIModelToRedis(ctx, modelID, userID, modelConfig); err != nil {
		return nil, err
	}

	return existingModel, nil
}

// DeleteCustomAIModel 删除自定义 AI 模型配置
func (s *AIServiceImpl) DeleteCustomAIModel(ctx context.Context, userID int64, modelID int64) error {
	if err := s.userAIRepo.DeleteCustomAIModel(ctx, modelID, userID); err != nil {
		return err
	}

	return s.repo.DeleteCustomAIModelFromRedis(ctx, modelID, userID)
}

// GetUserCurrentModelID 获取用户当前使用的模型 ID
func (s *AIServiceImpl) GetUserCurrentModelID(ctx context.Context, userID int64) (int64, error) {
	return s.userAIRepo.GetUserCurrentModelID(ctx, userID)
}

// UpdateUserCurrentModel 更新用户当前使用的模型 ID
func (s *AIServiceImpl) UpdateUserCurrentModel(ctx context.Context, userID int64, modelID int64) error {
	return s.userAIRepo.UpdateUserCurrentModel(ctx, userID, modelID)
}

// GetAIConfig 获取用户的 AI 配置
func (s *AIServiceImpl) GetAIConfig(ctx context.Context, userID int64) (*dto.AIConfigResponse, error) {
	// 获取当前模型 ID
	currentModelID, err := s.userAIRepo.GetUserCurrentModelID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 获取自定义模型列表
	customModels, err := s.userAIRepo.GetCustomAIModels(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 转换为响应格式
	customModelResponses := make([]dto.CustomAIModelResponse, len(customModels))
	for i, model := range customModels {
		customModelResponses[i] = dto.CustomAIModelResponse{
			ID:         model.ID,
			Name:       model.Name,
			Provider:   model.Provider,
			APIBaseURL: model.APIBaseURL,
			ModelName:  model.ModelName,
			IsActive:   model.IsActive,
			CreatedAt:  model.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	// 获取默认模型剩余次数
	remainingQuota := 10
	count, err := s.repo.GetAICallCount(ctx, userID)
	if err == nil {
		remainingQuota = 10 - count
		if remainingQuota < 0 {
			remainingQuota = 0
		}
	}

	return &dto.AIConfigResponse{
		CurrentModelID: currentModelID,
		RemainingQuota: remainingQuota,
		CustomModels:   customModelResponses,
	}, nil
}

func (s *AIServiceImpl) buildContextualPrompt(req *dto.AIChatRequest) string {
	var prompt string
	switch req.Page {
	case "automaton":
		if req.Automaton != nil {
			prompt = s.formatAutomatonForPrompt(req.Automaton)
		}
	case "grammar":
		if req.Grammar != nil {
			prompt = s.formatGrammarForPrompt(req.Grammar)
		}
	case "regex":
		if req.Regex != nil {
			prompt = fmt.Sprintf("当前正则表达式为：%s", *req.Regex)
		}
	default:
	}

	return prompt
}

func (s *AIServiceImpl) formatAutomatonForPrompt(a *model.Automaton) string {
	var buf strings.Builder
	buf.WriteString("当前的有限状态自动机定义如下：\n")

	// 状态集合
	states := make([]string, len(a.States))
	for i, st := range a.States {
		states[i] = string(st)
	}
	buf.WriteString("- 状态集合 Q：{" + strings.Join(states, ", ") + "}\n")

	// 字母表排除 ε
	alphabet := make([]string, 0, len(a.Alphabet))
	for _, sym := range a.Alphabet {
		if sym != model.Epsilon {
			alphabet = append(alphabet, string(sym))
		}
	}
	if len(alphabet) == 0 {
		buf.WriteString("- 输入字母表 Σ：∅（空集）\n")
	} else {
		buf.WriteString("- 输入字母表 Σ：{" + strings.Join(alphabet, ", ") + "}\n")
	}

	// 初始状态
	buf.WriteString("- 初始状态 q₀：" + string(a.InitialState) + "\n")

	// 接受状态
	if len(a.AcceptingStates) == 0 {
		buf.WriteString("- 接受状态集合 F：∅\n")
	} else {
		accepting := make([]string, len(a.AcceptingStates))
		for i, st := range a.AcceptingStates {
			accepting[i] = string(st)
		}
		buf.WriteString("- 接受状态集合 F：{" + strings.Join(accepting, ", ") + "}\n")
	}

	// 转移函数 δ
	buf.WriteString("- 状态转移规则 δ：\n")
	for _, t := range a.Transitions {
		input := string(t.Input)
		var target string
		if len(t.ToStates) == 1 {
			target = string(t.ToStates[0]) // 单状态不加花括号更自然
		} else {
			toStr := make([]string, len(t.ToStates))
			for i, to := range t.ToStates {
				toStr[i] = string(to)
			}
			target = "{" + strings.Join(toStr, ", ") + "}"
		}
		buf.WriteString(fmt.Sprintf("  δ(%s, %s) → %s\n", t.FromState, input, target))
	}

	return buf.String()
}

func (s *AIServiceImpl) formatGrammarForPrompt(g *model.Grammar) string {
	var buf strings.Builder
	buf.WriteString("当前文法定义如下：\n")

	// 起始符号
	buf.WriteString("- 起始符号 S：" + string(g.StartSymbol) + "\n")

	// 终结符
	terminals := make([]string, 0, len(g.Terminals))
	for _, t := range g.Terminals {
		if t == model.Epsilon {
			terminals = append(terminals, "ε")
		} else {
			terminals = append(terminals, string(t))
		}
	}
	if len(terminals) == 0 {
		buf.WriteString("- 终结符集合 T：∅\n")
	} else {
		buf.WriteString("- 终结符集合 T：{" + strings.Join(terminals, ", ") + "}\n")
	}

	// 非终结符
	nonTerminals := make([]string, len(g.NonTerminals))
	for i, nt := range g.NonTerminals {
		nonTerminals[i] = string(nt)
	}
	buf.WriteString("- 非终结符集合 N：{" + strings.Join(nonTerminals, ", ") + "}\n")

	// 产生式
	buf.WriteString("- 产生式规则 P：\n")
	for _, p := range g.Productions {
		left := make([]string, len(p.Left))
		for i, sym := range p.Left {
			left[i] = string(sym)
		}
		right := make([]string, len(p.Right))
		if len(p.Right) == 0 {
			right = []string{"ε"}
		} else {
			for i, sym := range p.Right {
				if sym == model.Epsilon {
					right[i] = "ε"
				} else {
					right[i] = string(sym)
				}
			}
		}
		buf.WriteString(fmt.Sprintf("  %s → %s\n", strings.Join(left, " "), strings.Join(right, " ")))
	}

	return buf.String()
}
