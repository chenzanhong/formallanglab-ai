要将当前基于 **SSE（Server-Sent Events）** 的 AI 聊天功能修改为使用 **WebSocket（WS）**，你需要对 **前端** 和 **后端** 都进行重构。以下是完整的迁移方案：

---

## ✅ 一、为什么用 WebSocket？

- SSE 是单向通信（服务端 → 客户端），而 WebSocket 是双向全双工。
- 如果未来需要客户端主动推送消息（比如中断请求、多轮交互控制等），WebSocket 更合适。
- 当前场景虽然只需要流式响应，但若你计划扩展功能（如取消生成、多模态输入等），WebSocket 更灵活。

---

## 🧱 二、整体架构变化

| 组件 | 原方案（SSE） | 新方案（WebSocket） |
|------|---------------|---------------------|
| 后端路由 | `POST /ai/sse` | `GET /ai/ws`（或 `ws://.../ai`） |
| 通信方式 | HTTP 流响应 | WebSocket 连接 |
| 消息格式 | 纯文本流（OpenAI 原始 chunk） | JSON 封装的消息（如 `{type: "chunk", data: "..."}`） |
| 前端调用 | `fetch + getReader()` | `new WebSocket()` + `onmessage` |

---

## 🔧 三、后端改造（Go）

### 1. 添加 WebSocket 路由和处理器

```go
// api/websocket.go
package handler

import (
	"github.com/chenzanhong/formallanglab-master/internal/domain/dto"
	"github.com/chenzanhong/formallanglab-master/internal/metrics"
	aiSvc "github.com/chenzanhong/formallanglab-master/internal/service/ai"
	"github.com/chenzanhong/formallanglab-master/logs"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源（生产环境应限制）
	},
}

type wsMessage struct {
	Type string `json:"type"` // "chunk", "error", "done"
	Data string `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func (h *AIhandler) AIChatWS(c *gin.Context) {
	start := time.Now()
	defer func() {
		metrics.ObserveOperationDuration("ai", "chat_ws", time.Since(start).Seconds())
	}()

	// 从查询参数或 header 获取 username（JWT 不适用于 WS 握手）
	username := c.Query("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "username is required", "result": false})
		return
	}

	// 接收初始请求体（首次消息）
	var req dto.AIChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid request body", "result": false})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zlog.Errorw("WebSocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	// 启动 AI 流处理
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, session, err := h.aiService.StreamChat(ctx, username, &req)
	if err != nil {
		conn.WriteJSON(wsMessage{Type: "error", Error: err.Error()})
		return
	}

	// 发送流数据到 WebSocket
	var aiResp strings.Builder
	for stream.Next() {
		content := stream.Current().Choices[0].Delta.Content
		aiResp.WriteString(content)
		if err := conn.WriteJSON(wsMessage{Type: "chunk", Data: content}); err != nil {
			zlog.Warnw("WebSocket write error", "error", err)
			break
		}
	}

	if err := stream.Err(); err != nil {
		conn.WriteJSON(wsMessage{Type: "error", Error: err.Error()})
	} else {
		conn.WriteJSON(wsMessage{Type: "done"})
	}

	// 异步保存会话
	go func() {
		session.RecentTurns = append(session.RecentTurns, model.QAPair{
			User: req.Question,
			AI:   aiResp.String(),
		})
		session.Trim()
		if err := h.aiService.SaveSession(context.Background(), username, session); err != nil {
			zlog.Warnw("AI会话保存失败", "detail", "无法保存用户会话信息")
		}
	}()
}
```

> ⚠️ 注意：WebSocket 握手是 GET 请求，不能直接传 JSON body。因此有两种选择：
> - **方案 A（推荐）**：前端先发一个 POST 获取 token/session，再用 token 连接 WS。
> - **方案 B（简化）**：首次 WS 消息包含完整请求（见下文前端实现）。

我们采用 **方案 B**：握手后，客户端立即发送第一条消息作为请求体。

但上面代码假设在握手前就拿到了 `req`，这在标准 WS 中不可行。所以更合理的做法是：

### ✅ 改进：握手后接收第一条消息作为请求

```go
func (h *AIhandler) AIChatWS(c *gin.Context) {
	// ... metrics ...

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 读取第一条消息作为请求
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		return
	}

	var req dto.AIChatRequest
	if err := json.Unmarshal(msgBytes, &req); err != nil {
		conn.WriteJSON(wsMessage{Type: "error", Error: "invalid request format"})
		return
	}

	// 从 JWT token 或消息中提取 username（这里假设消息含 username）
	// 更安全的方式：握手 URL 带 token，解析出 username
	username := c.Query("token") // 或从 req 中带 username（不推荐）
	if username == "" {
		conn.WriteJSON(wsMessage{Type: "error", Error: "missing username"})
		return
	}

	// ... 后续逻辑同上 ...
}
```

> 🔐 **安全建议**：通过 `?token=xxx` 传递 JWT，在服务端验证并提取 `username`。

---

## 💻 四、前端改造（React）

### 1. 修改 `aiChatSSE` 函数为 WebSocket 版本

```ts
// src/api/ai.ts
export const aiChatWS = (request: AIChatRequest): {
  reader: {
    read: () => Promise<{ done: boolean; value?: string; error?: string }>;
    cancel: () => void;
  };
  sendMessage: (msg: any) => void;
} => {
  const token = localStorage.getItem('FormalLangLab:accessToken');
  if (!token) throw new Error('未登录');

  // 构建 WebSocket URL
  const baseURL = apiClient.defaults.baseURL?.replace('http', 'ws') || 'ws://localhost:3000';
  const wsUrl = `${baseURL}/ai/ws?token=${encodeURIComponent(token)}`;

  const ws = new WebSocket(wsUrl);

  // 发送初始请求
  ws.onopen = () => {
    ws.send(JSON.stringify(request));
  };

  let resolveCurrent: ((value: string) => void) | null = null;
  let rejectCurrent: ((err: string) => void) | null = null;
  let buffer = '';

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data as string) as { type: string; data?: string; error?: string };
      if (msg.type === 'chunk') {
        buffer += msg.data || '';
        if (resolveCurrent) {
          resolveCurrent(buffer);
          resolveCurrent = null;
          rejectCurrent = null;
        }
      } else if (msg.type === 'done') {
        if (resolveCurrent) {
          resolveCurrent(buffer);
        }
        buffer = ''; // 可选
      } else if (msg.type === 'error') {
        if (rejectCurrent) {
          rejectCurrent(msg.error || '未知错误');
        }
      }
    } catch (e) {
      console.error('WebSocket 消息解析失败:', e);
    }
  };

  ws.onerror = (err) => {
    if (rejectCurrent) rejectCurrent('连接错误');
  };

  ws.onclose = () => {
    if (rejectCurrent) rejectCurrent('连接已关闭');
  };

  const reader = {
    read: (): Promise<{ done: boolean; value?: string; error?: string }> => {
      return new Promise((resolve, reject) => {
        if (buffer) {
          resolve({ done: false, value: buffer });
          buffer = '';
          return;
        }

        resolveCurrent = (value: string) => resolve({ done: false, value });
        rejectCurrent = (error: string) => resolve({ done: true, error });
      });
    },
    cancel: () => {
      ws.close();
    }
  };

  const sendMessage = (msg: any) => {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(msg));
    }
  };

  return { reader, sendMessage };
};
```

### 2. 修改 `AIAssistant.tsx` 中的调用逻辑

替换 `sendMessageToAI` 和流处理部分：

```tsx
const sendMessageToAI = async (message: string, context?: any) => {
  // ... 构建 aiChatRequest 同前 ...

  const { reader } = aiChatWS(aiChatRequest);
  return reader;
};

// 在 handleSendMessage 中：
try {
  const reader = await sendMessageToAI(inputMessage, context);

  setStreamingContent('');
  setIsStreaming(true);

  while (true) {
    const { done, value, error } = await reader.read();
    if (error) {
      throw new Error(error);
    }
    if (done) break;

    setStreamingContent(prev => prev + value);
  }

  // 成功结束
  setMessages(prev => [...prev, { id: Date.now().toString(), role: 'ai', content: streamingContentRef.current }]);
} catch (error) {
  // 错误处理同前
} finally {
  setIsStreaming(false);
  setStreamingContent('');
}
```

---

## 📝 五、Gin 路由注册

```go
// router.go
r := gin.Default()
aiHandler := NewAIHandler(aiService)

// 替换原来的 SSE 路由
r.GET("/ai/ws", aiHandler.AIChatWS) // 注意：是 GET，因为 WS 握手是 GET
```

---

## ✅ 六、优点与注意事项

### ✅ 优点
- 支持双向通信（未来可扩展“停止生成”等功能）
- 更低延迟（无 HTTP 头开销）
- 更适合长连接场景

### ⚠️ 注意事项
- **认证**：必须通过 URL 参数或首条消息传递身份，不能依赖 Cookie/JWT Header（除非自定义握手）
- **错误处理**：需监听 `onerror` 和 `onclose`
- **重连机制**：生产环境建议加入自动重连
- **代理配置**：Nginx 需配置 WebSocket 代理（`Upgrade` 和 `Connection` 头）

```nginx
location /ai/ws {
    proxy_pass http://backend;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

---

## 🔄 总结

| 步骤 | 操作 |
|------|------|
| 1 | 后端：用 `gorilla/websocket` 实现 `/ai/ws` 路由 |
| 2 | 前端：用 `new WebSocket()` 替代 `fetch` |
| 3 | 消息格式：封装为 `{type, data}` JSON |
| 4 | 认证：通过 URL 参数传递 token |
| 5 | 错误/完成：通过 `type: "error"` / `"done"` 通知前端 |


## 实现 “预置高频问题缓存” 来提升响应速度和用户体验
对某些固定问题（如“泵引理怎么证明 a^n b^n 不是正则语言？”），直接返回预设答案
缓存命中 → 不调用 AI 模型，秒回
缓存未命中 → 走原有逻辑
支持模糊匹配（比如用户多打空格/标点也能命中）