/*
AI模块 OpenAI Go SDK版本不低于 v2.4.0
*/
package handler

import (
	"ai/internal/domain/dto"
	"ai/internal/domain/model"
	metrics "ai/internal/middleware/metrics"
	"ai/internal/service"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chenzanhong/zlog"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type AIHandler struct {
	aiService service.AIService
	upgrader  websocket.Upgrader
}

func NewAIHandler(aiService service.AIService) *AIHandler {
	return &AIHandler{
		aiService: aiService,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源（生产环境应限制，从配置文件获取）
			},
		},
	}
}

func (h *AIHandler) AIChatSSE(c *gin.Context) {
	start := time.Now()
	defer func() {
		metrics.ObserveOperationDuration("ai", "chat_sse", time.Since(start).Seconds())
	}()

	var req dto.AIChatRequest
	if err := c.BindJSON(&req); err != nil {
		metrics.IncOperation("ai", "chat_sse", "failure: request body required")
		zlog.Warnw("AI对话请求失败", "detail", "请求体不能为空")
		c.JSON(http.StatusBadRequest, gin.H{"msg": "request body is required", "result": false})
		return
	}

	// JWT 中间件中Set的username
	username := c.GetString("username")
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "缺少有效的token"})
		return
	}

	// 设置SSE响应头
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	question := req.Question
	cachedAnswer := h.aiService.CheckCache(question)

	// 检查缓存
	if cachedAnswer != "" {
		metrics.IncOperation("ai", "cache_hit", "success")
		zlog.Infow("AI对话(SSE)缓存命中", "question", req.Question)

		// 模拟流式响应
		mockStream, err := h.aiService.MockStreamChat(c.Request.Context(), cachedAnswer)
		if err == nil {
			// 异步保存会话
			go func() {
				ctx := context.Background()
				session, err := h.aiService.GetSession(c.Request.Context(), username)
				if err != nil {
					metrics.IncOperation("ai", "chat_sse", "failure: get session error")
					zlog.Warnw("获取会话失败", "detail", "无法获取或创建用户会话")
					return
				}

				session.RecentTurns = append(session.RecentTurns, model.QAPair{
					User: req.Question,
					AI:   cachedAnswer,
				})
				session.Trim(h.aiService.GetMaxSessionTurns())
				if err := h.aiService.SaveSession(ctx, username, session); err != nil {
					metrics.IncOperation("ai", "chat_sse", "failure: save session error")
					zlog.Warnw("AI会话保存失败", "detail", "无法保存用户会话信息")
				}
			}()
			// 模拟流式响应
			for mockStream.Next() {
				chunk := mockStream.Current()
				if chunk == "" {
					continue
				}
				c.Writer.Write([]byte(chunk))
				c.Writer.Flush()
				// 模拟延迟（更像真实 AI）
				time.Sleep(100 * time.Millisecond)
			}
			return // 缓存命中，模拟流式响应完成，无需继续使用真实 ai 服务
		}
		// 缓存命中或获取模拟流，继续使用真实 ai 服务
	}

	metrics.IncOperation("ai", "cache_hit", "failure")
	zlog.Infow("AI对话(SSE)缓存未命中", "question", req.Question)

	session, err := h.aiService.GetSession(c.Request.Context(), username)
	if err != nil {
		metrics.IncOperation("ai", "chat_sse", "failure: get session error")
		zlog.Warnw("获取会话失败", "detail", "无法获取或创建用户会话")
		// 记录错误，但是不终止，允许不借助对话历史
	}
	stream, err := h.aiService.StreamChat(c.Request.Context(), session, &req)
	if err != nil {
		metrics.IncOperation("ai", "chat_sse", "failure: service error")
		zlog.Warnw("AI对话请求失败", "detail", "AI服务调用失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "result": false})
		return
	}

	// 用于记录完整 AI 响应（用于保存会话）
	var aiResp strings.Builder
	// 用于缓冲待发送的内容（提升性能）
	var buffer strings.Builder

	ticker := time.NewTicker(64 * time.Millisecond)
	defer ticker.Stop()

	done := make(chan bool)
	go func() {
		for {
			select {
			case <-ticker.C:
				if buffer.Len() > 0 {
					// 写入客户端
					// 不使用c.SSE，ai响应本身就是流式，无需再SSE
					c.Writer.Write([]byte(buffer.String()))
					c.Writer.Flush()
					// 同步到完整响应记录
					aiResp.WriteString(buffer.String())
					buffer.Reset()
				}
			case <-done:
				return
			}
		}
	}()

	for stream.Next() {
		buffer.WriteString(stream.Current().Choices[0].Delta.Content)
	}

	// 停止ticker并关闭done通道
	ticker.Stop()
	close(done)

	// 最终 flush 剩余内容
	if buffer.Len() > 0 {
		c.Writer.Write([]byte(buffer.String()))
		c.Writer.Flush()
		aiResp.WriteString(buffer.String())
		buffer.Reset()
	}

	if err := stream.Err(); err != nil {
		metrics.IncOperation("ai", "chat_sse", "failure: stream error")
		zlog.Warnw("AI流式传输失败", "detail", "流式传输过程中发生错误")
	}
	// fmt.Println(aiResp)

	go func() {
		ctx := context.Background()
		session.AddTurns(req.Question, aiResp.String())
		// session.RecentTurns = append(session.RecentTurns, model.QAPair{
		// 	User: req.Question,
		// 	AI:   aiResp.String(),
		// })
		session.Trim(h.aiService.GetMaxSessionTurns())
		if err := h.aiService.SaveSession(ctx, username, session); err != nil {
			metrics.IncOperation("ai", "chat_sse", "failure: save session error")
			zlog.Warnw("AI会话保存失败", "detail", "无法保存用户会话信息")
		}
	}()

	metrics.IncOperation("ai", "chat_sse", "success")
	zlog.Infow("AI对话(SSE)请求成功")
}

// ================ WebSocket ================

func (h *AIHandler) AIChatWS(c *gin.Context) {
	fmt.Println("=============== AIChatWS ================")
	start := time.Now()
	defer func() {
		metrics.ObserveOperationDuration("ai", "chat_ws", time.Since(start).Seconds())
	}()

	if c.Request.Header.Get("Upgrade") != "websocket" {
		metrics.IncOperation("ai", "chat_ws", "failure: upgrade required")
		zlog.Warnw("AI对话请求失败", "detail", "升级为WebSocket协议失败")
		c.JSON(http.StatusBadRequest, gin.H{"msg": "upgrade required", "result": false})
		return
	}

	username := c.GetString("username")
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "unauthorized", "result": false})
		return
	}

	// 升级HTTP连接为WebSocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zlog.Errorw("WebSocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()
	fmt.Println(c.Request.URL, " ", c.Request.Host)

	var writeMu sync.Mutex

	// 封装安全写函数，因为 WebSocket 的 conn 不是并发安全的
	safeWrite := func(msg model.WsMessage) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		// 设置写超时：10分钟内必须回复，否则断开
		conn.SetWriteDeadline(time.Now().Add(10 * time.Minute))
		return conn.WriteJSON(msg)
	}

	type StreamControl struct {
		cancel context.CancelFunc
		done   chan struct{}
	}

	var (
		mu            sync.Mutex
		currentStream *StreamControl
	)

	// 连接关闭时确保清理
	defer func() {
		mu.Lock()
		if currentStream != nil {
			currentStream.cancel()
			<-currentStream.done
		}
		mu.Unlock()
	}()

	// 在 conn 成功 upgrade 后
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := safeWrite(model.WsMessage{Type: model.MsgTypePing}); err != nil {
					return // 连接已断
				}
			case <-c.Request.Context().Done(): // 可关联到连接生命周期
				return
			}
		}
	}()

	// 主消息循环
	for {
		// 设置读超时：5分钟内必须发消息，否则断开
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				zlog.Warnw("WebSocket read message failed", "error", err)
			}
			return // 客户端断开，退出循环
		}

		var incoming model.WsMessage
		if err := json.Unmarshal(msgBytes, &incoming); err != nil {
			// fmt.Println(1)
			safeWrite(model.WsMessage{Type: model.MsgTypeError, Error: "invalid JSON format"})
			continue
		}

		switch incoming.Type {
		case model.MsgTypeChat:
			// 如果已有流在运行，先取消它
			mu.Lock()
			if currentStream != nil {
				currentStream.cancel()
				<-currentStream.done
			}

			// 处理聊天消息
			var req dto.AIChatRequest
			if err := json.Unmarshal([]byte(incoming.Data), &req); err != nil {
				// fmt.Println(2)
				safeWrite(model.WsMessage{Type: model.MsgTypeError, Error: "invalid JSON format"})
				continue
			}

			// 创建新的上下文和取消函数
			ctx, cancel := context.WithCancel(context.Background())
			doneChan := make(chan struct{})
			currentStream = &StreamControl{
				cancel: cancel,
				done:   doneChan,
			}
			mu.Unlock()

			qaCacheAnswer := h.aiService.CheckCache(req.Question)
			// 判断是否命中预置缓存
			if qaCacheAnswer != "" {
				fmt.Println("\nWebSocket 命中缓存，问题是：", req.Question)
				metrics.IncOperation("ai", "cache_hit", "success")
				mockStream, err := h.aiService.MockStreamChat(ctx, qaCacheAnswer)
				if err == nil {
					// 异步保存会话
					go func() {
						session, err := h.aiService.GetSession(ctx, username)
						if err != nil {
							metrics.IncOperation("ai", "chat_ws", "failure: get session error")
							zlog.Errorw("Get session failed", "error", err)
							return
						}
						session.AddTurns(req.Question, qaCacheAnswer)
						session.Trim(h.aiService.GetMaxSessionTurns())
						if saveErr := h.aiService.SaveSession(context.Background(), username, session); saveErr != nil {
							metrics.IncOperation("ai", "chat_ws", "failure: save session error")
							zlog.Errorw("Save session failed", "error", saveErr)
						}
					}()
					// 启动异步模拟流处理
					go func() {
						defer close(doneChan) // 通知主 goroutine: 我已完成
						for mockStream.Next() {
							// 检查是否被取消
							select {
							case <-ctx.Done():
								// fmt.Println(4)
								safeWrite(model.WsMessage{Type: model.MsgTypeStopped})
								return
							default:
								// 继续发送chuck
							}
							content := mockStream.Current()

							conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
							// fmt.Println(5)
							time.Sleep(100 * time.Millisecond) // 模拟ai延迟
							if err := safeWrite(model.WsMessage{Type: model.MsgTypeChunk, Data: content}); err != nil {
								zlog.Warnw("WebSocket write message failed", "error", err)
								return
							}
						}

						// 流结束
						if err := mockStream.Err(); err != nil {
							// fmt.Println(6)
							safeWrite(model.WsMessage{Type: model.MsgTypeError, Error: err.Error()})
						} else {
							// fmt.Println(7)
							safeWrite(model.WsMessage{Type: model.MsgTypeDone})
						}
						// fmt.Println("AI对话（MockStreamChat）请求成功")
						fmt.Println("MockAI 回答：", qaCacheAnswer)
					}()
					continue
				}
			}

			metrics.IncOperation("ai", "cache_hit", "failure")
			// 未命中缓存/MockStreamChat失败，升级到 ai 服务
			// 异步启动 AI 流
			go func() {
				defer close(doneChan) // 通知主 goroutine: 我已完成
				// fmt.Printf("%s\n%+v", req.Question, req.Automaton)
				session, err := h.aiService.GetSession(ctx, username)
				if err != nil {
					metrics.IncOperation("ai", "chat_ws", "failure: get session error")
					zlog.Errorw("Get session failed", "error", err)
					safeWrite(model.WsMessage{Type: model.MsgTypeError, Error: err.Error()})
					return
				}
				stream, err := h.aiService.StreamChat(ctx, session, &req)
				if err != nil {
					metrics.IncOperation("ai", "chat_ws", "failure: stream chat error")
					zlog.Errorw("Stream chat failed", "error", err)
					safeWrite(model.WsMessage{Type: model.MsgTypeError, Error: err.Error()})
					return
				}

				var aiResp strings.Builder
				for stream.Next() {
					// 检查是否被取消
					select {
					case <-ctx.Done():
						// fmt.Println(4)
						safeWrite(model.WsMessage{Type: model.MsgTypeStopped})
						return
					default:
						// 继续发送chuck
					}
					content := stream.Current().Choices[0].Delta.Content
					// 下面三个参数有些ai不会携带
					fmt.Println("CompletionTokens: ", stream.Current().Usage.CompletionTokens)
					fmt.Println("PromptTokens: ", stream.Current().Usage.PromptTokens)
					fmt.Println("TotalTokens: ", stream.Current().Usage.TotalTokens)
					// fmt.Println(content)
					aiResp.WriteString(content)

					conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
					// fmt.Println(5)
					if err := safeWrite(model.WsMessage{Type: model.MsgTypeChunk, Data: content}); err != nil {
						zlog.Warnw("WebSocket write message failed", "error", err)
						return
					}
				}

				// 流结束
				if err := stream.Err(); err != nil {
					// fmt.Println(6)
					safeWrite(model.WsMessage{Type: model.MsgTypeError, Error: err.Error()})
				} else {
					// fmt.Println(7)
					safeWrite(model.WsMessage{Type: model.MsgTypeDone})
				}
				// fmt.Println("AI对话（StreamChat）请求成功")
				fmt.Println("AI 回答：", aiResp.String())
				// 异步保存会话
				go func() {
					// session.RecentTurns = append(session.RecentTurns, model.QAPair{
					// 	User: req.Question,
					// 	AI:   aiResp.String(),
					// })
					session.AddTurns(req.Question, aiResp.String())
					session.Trim(h.aiService.GetMaxSessionTurns())
					if saveErr := h.aiService.SaveSession(context.Background(), username, session); saveErr != nil {
						metrics.IncOperation("ai", "chat_ws", "failure: save session error")
						zlog.Warnw("AI会话保存失败", "detail", "无法保存用户会话信息")
					}
				}()
			}()
		case model.MsgTypeStop:
			// 处理停止消息
			mu.Lock()
			if currentStream != nil {
				currentStream.cancel()
				// 不等待done，立即响应
			}
			mu.Unlock()
			// fmt.Println(8)
			safeWrite(model.WsMessage{Type: model.MsgTypeStopped})
		case model.MsgTypePing:
			// 处理心跳消息
			// fmt.Println(9)
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			safeWrite(model.WsMessage{Type: model.MsgTypePong})
		default:
			// fmt.Println(10)
			safeWrite(model.WsMessage{Type: model.MsgTypeError, Error: "unknown message type"})
		}
	}

}
