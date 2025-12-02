### 3. **AI 模块（可选）**

| 功能 | 技术推荐 | 说明 |
|------|-----------|------|
| 本地运行 | **Ollama + Llama3 / Phi3 / Mistral** | 本地部署小型语言模型 |
| API 调用 | **阿里云 Qwen API / OpenAI API / 百度千帆 / Moonshot** | 使用成熟大模型服务 |
| 封装接口 | **自定义中间层封装 AI 请求逻辑** | 对接前端与后端，返回自然语言结果 |
| 示例功能 | **文法解释、错误修正、自动补全、练习题生成** | 提升系统智能化程度 |

步骤	操作
1️⃣ 选择模型	下载 Qwen 或 DeepSeek 开源模型
2️⃣ 构建数据	收集/生成形式语言题目和答案
3️⃣ 购买算力	租用云GPU（如 AutoDL）
4️⃣ 微调模型	使用 LoRA 微调，让模型学会形式语言与自动机
5️⃣ 部署使用	封装成 API 或本地运行

核心设计理念
AI Copilot 不是外挂的聊天窗口，而是系统的“智能大脑”，它能：

🔍 自动感知上下文（用户正在操作的自动机、输入的字符串、当前页面状态）
🧠 构造专业 Prompt（将结构化数据转为高质量提示词）

使用React-Chatbot-Kit 实现ai对话组件，后端调用通义千问模型。
但是目前实现没用到React-Chatbot-Kit，效果也还行。
Dey, A. K. (2001). Understanding and Using Context. Personal and Ubiquitous Computing, 5(1), 4–7.
👉 提出“上下文 = 用户 + 环境 + 任务”，你通过 getCurrentContext() 获取路由和页面数据，正是一种轻量级上下文感知实现。
---


评委：“所以你的 AI 模块就是单纯调用了一下通义千问的接口，对吧？”

你（自信、清晰）：

“是的，底层确实调用了 Qwen 的 API，但关键不在于调用谁，而在于我们如何让这个通用模型变成一个‘懂自动机的教学助手’。

我们做了三件事：

上下文感知：系统会自动把用户当前操作的自动机结构（比如这是一个含 ε 转移的 NFA）、输入的字符串、错误日志等结构化数据注入到提示词中；
教学约束：通过 system prompt 限制 AI 只回答形式语言相关问题，并要求用教学语言（如‘请分步骤说明’‘避免使用未定义术语’）；
功能聚焦：AI 不是闲聊机器人，而是专门用于错误诊断、概念澄清和练习引导——这三点都是教学痛点。
换句话说，我们不是在‘接一个聊天窗口’，而是在构建一个‘情境化教学代理’。”


这是一个非常关键的问题！“如何存储上下文并在用户提问时注入 AI Prompt” 正是你把“普通 API 调用”升级为“智能教学助手”的核心技术点。

下面我为你提供一套轻量、可靠、与你的 Go + React 系统无缝集成的上下文管理方案，无需复杂状态管理，1 天内即可实现。

🎯 目标
用户在前端操作自动机（如编辑 DFA、测试字符串）
点击“AI 助教”时，自动携带当前页面状态（如 DFA 结构、错误信息）
后端将这些信息结构化注入 Prompt，调用 Qwen API
AI 返回情境化回答

✅ 一、上下文存储：前端（React + TypeScript）
✨ 核心思想：不额外存储，直接从当前组件状态提取

你已经在前端维护了自动机的状态（比如 Automaton: automaton），无需单独“存储上下文”，只需在提问时序列化当前状态即可。
示例：前端发送 AI 请求
const response = await fetch('/api/ai/ask', {
method: 'POST',
headers: { 'Content-Type': 'application/json' },
body: JSON.stringify({
question: userQuestion,
context: context, // 👈 关键：携带上下文，从当前页面输入款获取
}),
});
✅ 优点：
无额外存储负担
上下文始终与 UI 状态一致
不依赖 localStorage 或全局状态（除非你用了 Redux/Zustand，那更简单）

✅ 二、上下文处理：后端（Go）
步骤：
1. 接收前端传来的 context（JSON）
2. 将结构化数据转换为自然语言描述
3. 拼接到 Prompt 中
示例：Go 后端处理
go
// ai/ai_service.go
type AIRequest struct {
Question string json:"question"
Context AIContext json:"context"
}

type AIContext struct {
automaton automaton json:"automaton,omitempty"
LastTest TestResult json:"lastTest,omitempty"
Page string json:"currentPage,omitempty"
}

type automaton struct {
Type string json:"type"
States []string json:"states"
Alphabet []string json:"alphabet"
Transitions []Transition json:"transitions"
Start string json:"start"
Accepts []string json:"accepts"
}

// 将结构化上下文转为自然语言描述
func (c *AIContext) ToPromptString() string {
var parts []string

if c.automaton != nil {
a := c.automaton
parts = append(parts, fmt.Sprintf(
"用户当前正在操作一个 %s，包含 %d 个状态：%v，字母表为 %v，起始状态是 %s，接受状态是 %v。",
a.Type, len(a.States), a.States, a.Alphabet, a.Start, a.Accepts,
))
// 可选：摘要转移函数（避免太长）
if len(a.Transitions) <= 5 {
parts = append(parts, "转移函数为：")
for _, t := range a.Transitions {
parts = append(parts, fmt.Sprintf(" %s --%s--> %s", t.From, t.Input, t.To))
}
}
}

if c.LastTest != nil {
result := "未被接受"
if c.LastTest.Accepted {
result = "被接受"
}
parts = append(parts, fmt.Sprintf("用户最近测试的字符串 \"%s\" %s。", c.LastTest.Input, result))
}

return strings.Join(parts, " ")
}

// 构造最终 Prompt
func BuildPrompt(userQuestion string, context AIContext) string {
systemPrompt := "你是一个形式语言与自动机课程的教学助手。请根据以下上下文，用简洁、准确、教学友好的语言回答学生的问题。避免使用未定义的术语，必要时分步骤说明。"

contextStr := context.ToPromptString()
if contextStr == "" {
contextStr = "无特定上下文。"
}

return fmt.Sprintf(
"%s\n\n【当前上下文】\n%s\n\n【学生问题】\n%s",
systemPrompt,
contextStr,
userQuestion,
)
}

✅ 三、调用 AI API（Go）

go
func AskQwen(prompt string) (string, error) {
// 构造 Qwen 请求（略，见前文）
// 发送请求
// 解析响应，返回 content
}

✅ 四、完整流程示例
用户操作：
1. 在前端构建一个 DFA：q0 --a--> q1, q1 --b--> q2，接受状态 {q2}
2. 测试字符串 "ab" → 被接受 ✅
3. 测试字符串 "a" → 未被接受 ❌
4. 点击“AI 助教”，输入：“为什么 'a' 不被接受？”

后端生成 Prompt：

你是一个形式语言与自动机课程的教学助手。请根据以下上下文，用简洁、准确、教学友好的语言回答学生的问题。避免使用未定义的术语，必要时分步骤说明。

【当前上下文】
用户当前正在操作一个 DFA，包含 3 个状态：[q0 q1 q2]，字母表为 [a b]，起始状态是 q0，接受状态是 [q2]。转移函数为：
q0 --a--> q1
q1 --b--> q2
用户最近测试的字符串 "a" 未被接受。

【学生问题】
为什么 'a' 不被接受？
AI 可能的回答：
“因为字符串 'a' 的识别路径是：从 q0 读入 'a' 到达 q1，但 q1 不是接受状态。您的 DFA 只有在读完 'ab' 到达 q2 时才接受。若希望接受 'a'，可将 q1 加入接受状态。”

✅ 五、优化建议（可选）

优化点 说明
------- ------
上下文摘要 如果自动机很大（>10 状态），只传摘要（如“5 状态 DFA，含冗余状态”）
错误日志注入 如果字符串识别失败，记录模拟路径（如“在 q1 遇到输入 'b' 无转移”）
缓存机制 对相同上下文+问题缓存 AI 回答，节省 API 调用

✅ 六、答辩时如何展示？

在 PPT 中画一张图：

[前端] → (当前自动机 + 测试结果) → [后端] → (结构化上下文 → 自然语言 Prompt) → [Qwen API] → [情境化回答]

并说：
“我们的 AI 不是孤立的聊天框，而是深度嵌入教学流程。系统会自动把学生正在操作的自动机‘告诉’AI，让回答从‘泛泛而谈’变为‘精准指导’。”

💡 总结
不需要额外存储上下文：直接从前端状态提取
后端做结构化 → 自然语言转换：这是你的核心设计
Prompt 工程 = 教学设计：约束 AI 行为，提升教学价值


为什么“不推荐记录聊天历史”？
原因	说明
1. 教学场景本质是“任务导向”	学生问：“为什么这个 DFA 不接受 'ab'？” → 得到答案 → 问题解决。极少需要多轮追问（如“那怎么改？”“如果加个状态呢？”），即使有，也可视为新问题。
2. 上下文漂移风险高	如果把历史对话全喂给 AI，容易导致：
AI 混淆当前自动机状态（比如用户已切换到另一个 DFA，但 AI 还在回答上一个）
回答偏离当前页面内容 | | 3. 增加开发与维护成本 | 需管理对话 ID、清理过期上下文、处理并发等，对毕设而言 ROI（投入产出比）极低 | | 4. 答辩演示更可控 | 单轮问答更容易展示“上下文感知”价值；多轮对话容易翻车（AI 胡说） |


正如 Krstić 等人（2022）在《Artificial Intelligence in Education: A Review》中指出，人工智能在教育中的核心价值之一是“为学生提供个性化的指导、支持或反馈”[1]。本系统集成的 AI 问答模块，正是这一理念的具体实践：它能够感知用户当前正在编辑的形式语言模型（如自动机或文法），并基于此上下文生成针对性解释，从而在无人干预的情况下，为学生提供近似一对一辅导的认知支持，有效缓解自主学习中的理解障碍。










要快速训练一个专门处理“形式语言与自动机”领域的 AI 模型，并不需要从零开始训练大模型（成本高、周期长），而是可以采用 “微调（Fine-tuning）”或“提示工程（Prompt Engineering）+ 检索增强（RAG）” 的高效策略。

下面为你提供 三种由快到慢、由轻量到专业的方案，你可以根据毕设时间、算力资源和效果需求选择：

✅ 方案一：最快（1 天内完成）｜RAG + 通用大模型（推荐！）
无需训练模型，只需构建知识库 + 调用 API
原理：
将《形式语言与自动机》教材、课件、典型例题、常见错误解析等整理为 结构化文本知识库
用户提问时，先用 向量检索 找出最相关的知识点
将知识点 + 用户问题 一起输入给 通义千问（Qwen）等大模型
模型基于专业上下文生成精准回答
优势：
⏱️ 1 天可上线
💰 成本极低（仅 API 调用费）
🎯 准确率高（避免大模型“胡说八道”）
🔧 无需 GPU
实现步骤：
1. 准备知识库（约 50–200 篇文档）：
教材章节（如蒋宗礼《形式语言与自动机》PDF 转文本）
典型 DFA/NFA 构造题及解析
常见误区（如“ε-闭包计算错误”）
自动机转换步骤（NFA→DFA、CFG→PDA 等）

2. 嵌入向量化：
使用开源模型（如 text2vec-base-chinese 或 bge-large-zh）将文档分块并生成向量
存入向量数据库（如 ChromaDB、FAISS）

3. 构建 RAG 流程：
python
用户提问 → 向量检索 top-3 相关段落 → 拼接 prompt → 调用 Qwen API → 返回答案

4. 前端集成：React 调用你的后端 /ai/sse 接口，支持流式输出
效果示例：
用户问：“如何将正则表达式 (a b)*abb 转为 DFA？”

RAG 检索到：“Thompson 构造法步骤...” + “子集构造法示例...”

Qwen 基于这些内容生成步骤清晰、带状态图描述的回答

✅ 这是目前最推荐的方案，适合毕设！

✅ 方案二：较快（1–2 周）｜微调开源小模型（如 Qwen-1.8B-Chat）
在通用模型基础上，用专业数据“教”它自动机知识
适用场景：
希望完全本地部署（不依赖 API）
有 GPU（如 RTX 3090 / 4090 或租用云 GPU）
步骤：
1. 准备训练数据（格式：{"instruction": "问题", "output": "专业回答"}）
示例：
json
{
"instruction": "DFA 和 NFA 的区别是什么？",
"output": "DFA 每个状态对每个输入符号有且仅有一个转移；NFA 允许多个转移或 ε 转移..."
}
数据量建议：500–2000 条高质量问答对

2. 选择基础模型：
中文推荐：Qwen-1.8B-Chat、ChatGLM3-6B（较小，可微调）
英文推荐：Mistral-7B-Instruct

3. 使用 LoRA 微调（低资源高效微调）：
工具：unsloth + trl + peft
代码库：Hugging Face Transformers

4. 部署推理服务（FastAPI + vLLM）
优势：
回答更稳定、专业
可离线运行
劣势：
需要 GPU 和一定深度学习基础
微调后仍可能“幻觉”，需结合规则校验

✅ 方案三：长期（学术研究级）｜构建领域大模型（不推荐毕设使用）
从头预训练 + 领域继续预训练 + 指令微调
需要数万条语料、多卡 GPU、数周训练
适合 PhD 或企业项目，不适合本科毕设

📌 综合建议（针对你的毕设）

你的需求 推荐方案
-------- --------
时间紧（1–2 个月） ✅ 方案一：RAG + Qwen API
想展示“本地 AI 能力” ✅ 方案二：微调 Qwen-1.8B（如有 GPU）
不想碰模型训练 ✅ 纯 RAG + 提示词优化
💡 强烈建议采用方案一（RAG），它：
能在答辩时展示“专业问答”效果
技术栈现代（向量检索 + LLM）
完全匹配你“Web + Go + React”的架构
可作为未来扩展个性化 AI 的基础

🔧 附：RAG 快速启动资源
向量模型（中文）：
BAAI/bge-large-zh-v1.5（Hugging Face）
向量库：
ChromaDB（简单）、FAISS（高效）
后端框架：
Go 调用 Qwen API + ChromaDB（可用 go-openai + REST）
前端：
React + SSE（Server-Sent Events）实现流式对话

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
)

const (
	CHROMA_URL      = "http://localhost:8000" // ChromaDB 默认端口
	DASHSCOPE_API_KEY = "your-dashscope-api-key"
)

// Chroma 查询请求结构
type ChromaQuery struct {
	CollectionName string   `json:"collection_name"`
	QueryTexts     []string `json:"query_texts"`
	NResults       int      `json:"n_results"`
}

// Chroma 响应结构（简化）
type ChromaResponse struct {
	Documents [][]string `json:"documents"`
}

func queryChroma(query string) ([]string, error) {
	payload := ChromaQuery{
		CollectionName: "automata_knowledge",
		QueryTexts:     []string{query},
		NResults:       3,
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(CHROMA_URL+"/api/v1/query", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var chromaResp ChromaResponse
	if err := json.NewDecoder(resp.Body).Decode(&chromaResp); err != nil {
		return nil, err
	}

	if len(chromaResp.Documents) > 0 {
		return chromaResp.Documents[0], nil
	}
	return []string{}, nil
}

func aiChatSSE(c *gin.Context) {
	var req struct {
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid JSON"})
		return
	}

	// 1. 向量检索
	relevantDocs, err := queryChroma(req.Message)
	if err != nil {
		log.Printf("Chroma query error: %v", err)
		relevantDocs = []string{} // 降级：无检索结果
	}

	// 2. 构造 prompt
	context := ""
	for _, doc := range relevantDocs {
		context += doc + "\n"
	}

	systemPrompt := `你是一位形式语言与自动机课程的助教，回答需准确、简洁、专业。
请基于以下参考资料回答问题。若参考资料无关，请仅基于你的知识回答，但不要编造。

参考资料：
` + context

	// 3. 调用 Qwen（流式）
	client := openai.NewClientWithConfig(openai.ClientConfig{
		BaseURL:    "https://dashscope.aliyuncs.com/compatible-mode/v1",
		APIKey:     DASHSCOPE_API_KEY,
		HTTPClient: &http.Client{},
	})

	stream, err := client.CreateChatCompletionStream(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: "qwen-max", // 或 qwen-plus
			Messages: []openai.ChatCompletionMessage{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: req.Message},
			},
			Stream: true,
		},
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer stream.Close()

	// 4. 流式返回（SSE）
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Stream error: %v", err)
			break
		}

		for _, choice := range response.Choices {
			if choice.Delta.Content != "" {
				fmt.Fprintf(c.Writer, "data: %s\n\n", choice.Delta.Content)
				c.Writer.Flush()
			}
		}
	}
}

func main() {
	r := gin.Default()
	r.POST("/ai/sse", aiChatSSE)
	r.Run(":8080")
}