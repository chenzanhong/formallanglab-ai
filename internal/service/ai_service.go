package service

import (
	"ai/configs"
	"ai/internal/core"
	"ai/internal/domain/dto"
	"ai/internal/domain/model"
	"ai/internal/repository"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/ssestream"
)

const (
	systemPrompt = `你是一位形式语言与自动机理论课程的教学助教，专注于自动机、正则表达式、文法和图灵机等内容。
	
目前和学生一对一交流，回答中不要用“你们”，而应该用“你”。

请用清晰、准确、循序渐进的方式回答问题，优先解释原理和“为什么”，必要时举例说明。

如果学生提供了自动机结构、文法或测试字符串，请结合具体情境以及已有对话内容进行分析。

不要虚构定理或算法，不确定时请说明“标准理论中通常……”。

注意！对于明显超出形式语言与自动机范围的问题（如编程、其他课程内容等），请礼貌回应，例如：
> “抱歉，这个问题超出了本课程《形式语言与自动机》的范围，我无法回答该问题。”
`
)

type AIService interface {
	StreamChat(ctx context.Context, session *model.AISession, req *dto.AIChatRequest) (*ssestream.Stream[openai.ChatCompletionChunk], error)
	MockStreamChat(ctx context.Context, cacheAnswer string) (*model.MockStream, error)
	CheckCache(question string) string // 查看是否命中预置高频问题缓存
	GetSession(ctx context.Context, username string, page core.PageType) (*model.AISession, error)
	SaveSession(ctx context.Context, username string, session *model.AISession) error
	GetSessionExpireSeconds() int
	GetMaxSessionTurns() int
}

type AIServiceImpl struct {
	client  *openai.Client
	repo    repository.AIRepository
	qaCache *model.QACache // 显式依赖
	aiCfg   configs.AIConfig
}

func NewAIService(
	client *openai.Client,
	repo repository.AIRepository,
	qaCache *model.QACache, // 新增参数
	aiCfg configs.AIConfig,
) AIService {
	return &AIServiceImpl{
		client:  client,
		repo:    repo,
		qaCache: qaCache,
		aiCfg:   aiCfg,
	}
}

// StreamChat 处理流式对话请求
func (s *AIServiceImpl) StreamChat(ctx context.Context, session *model.AISession, req *dto.AIChatRequest) (*ssestream.Stream[openai.ChatCompletionChunk], error) {
	// 构造消息：严格遵循 [system] → [history] → [context] → [current question]
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
	}

	// 历史对话：直接交替添加
	for _, turn := range session.RecentTurns {
		// fmt.Println(turn.AI)
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
	stream := s.client.Chat.Completions.NewStreaming(
		ctx, openai.ChatCompletionNewParams{
			Messages: messages,
			Model:    os.Getenv("DASHSCOPE_MODEL"),
		},
	)

	return stream, nil
}

// MockStreamChat 模拟流式响应，用于响应预置高频问题缓存
func (s *AIServiceImpl) MockStreamChat(ctx context.Context, cacheAnswer string) (*model.MockStream, error) {
	return model.NewMockStream(cacheAnswer), nil
}

// CheckCache 查看是否命中预置高频问题缓存
func (s *AIServiceImpl) CheckCache(question string) string {
	return s.qaCache.Get(question)
}

// SaveSession 保存会话（不变）
func (s *AIServiceImpl) SaveSession(ctx context.Context, username string, session *model.AISession) error {
	return s.repo.SaveSession(ctx, username, session, s.aiCfg.SessionExpireSeconds)
}

func (s *AIServiceImpl) GetSession(ctx context.Context, username string, page core.PageType) (*model.AISession, error) {
	// 1. 从redis加载会话
	session, err := s.repo.GetSession(ctx, username, string(page))
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}
	if session == nil {
		session = &model.AISession{Page: page}
	}
	session.LastActive = time.Now().Unix()
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
	buf.WriteString("当前上下文无关文法（CFG）定义如下：\n")

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
