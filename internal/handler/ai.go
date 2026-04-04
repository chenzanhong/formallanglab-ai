/*
AI模块 OpenAI Go SDK版本不低于 v2.4.0
*/
package handler

import (
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

	"github.com/chenzanhong/formallanglab-ai/internal/domain/dto"
	"github.com/chenzanhong/formallanglab-ai/internal/domain/model"
	metrics "github.com/chenzanhong/formallanglab-ai/internal/middleware/metrics"
	"github.com/chenzanhong/formallanglab-ai/internal/service"
)

const (
	defaultModelID     = int64(0)
	streamFlushTimeout = 1 * time.Second
	mockStreamDelay    = 100 * time.Millisecond

	wsHeartbeatInterval = 30 * time.Second
	wsReadTimeout       = 5 * time.Minute
	wsMaxWriteTimeout   = 10 * time.Minute
)

type AIHandler struct {
	aiService service.AIService
	upgrader  websocket.Upgrader
}

var allowedOrigins = map[string]bool{
	"https://caohaitong.xyz": true,
	"http://caohaitong.xyz":  true,
	"http://113.44.170.52":   true,
	"https://113.44.170.52":  true,
	"http://localhost:5173":  true,
	"http://localhost:3000":  true,
}

func NewAIHandler(aiService service.AIService) *AIHandler {
	return &AIHandler{
		aiService: aiService,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return allowedOrigins[origin]
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
		zlog.Warnw("AI 对话请求失败", "detail", "请求体不能为空")
		c.JSON(http.StatusBadRequest, gin.H{"msg": "request body is required", "result": false})

		return
	}

	username := c.GetString("username")
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "缺少有效的 token"})
		return
	}

	userID := c.GetInt64("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "缺少用户 ID"})
		return
	}

	modelID := defaultModelID
	if modelIDStr := c.Query("model_id"); modelIDStr != "" {
		if _, err := fmt.Sscanf(modelIDStr, "%d", &modelID); err != nil {
			modelID = defaultModelID
		}
	}

	if modelID == defaultModelID {
		canCall, err := h.aiService.CheckAICallLimit(c.Request.Context(), userID)
		if err != nil {
			metrics.IncOperation("ai", "chat_sse", "failure: check call limit error")
			zlog.Warnw("检查 AI 调用限制失败", "detail", "无法检查用户调用限制")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "检查调用限制失败", "result": false})

			return
		}
		if !canCall {
			metrics.IncOperation("ai", "chat_sse", "failure: call limit exceeded")
			zlog.Warnw("AI 调用次数已达今日上限", "username", username)
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "AI 调用次数已达今日上限，请明日再试或使用自定义模型", "result": false})

			return
		}
		if err := h.aiService.IncrementAICallCount(c.Request.Context(), userID); err != nil {
			metrics.IncOperation("ai", "chat_sse", "failure: increment call count error")
			zlog.Warnw("增加 AI 调用计数失败", "detail", "无法更新用户调用计数")
		}
	}

	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	cachedAnswer := h.aiService.CheckCache(req.Question)
	if cachedAnswer != "" {
		metrics.IncOperation("ai", "cache_hit", "success")
		zlog.Infow("AI 对话 (SSE) 缓存命中", "question", req.Question)

		mockStream, err := h.aiService.MockStreamChat(c.Request.Context(), cachedAnswer)
		if err == nil {
			go func() {
				ctx := context.Background()
				session, err := h.aiService.GetSession(ctx, username)
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
				if saveErr := h.aiService.SaveSession(ctx, username, session); saveErr != nil {
					metrics.IncOperation("ai", "chat_sse", "failure: save session error")
					zlog.Warnw("AI 会话保存失败", "detail", "无法保存用户会话信息")
				}
			}()

			for mockStream.Next() {
				chunk := mockStream.Current()
				if chunk == "" {
					continue
				}
				if _, err := c.Writer.Write([]byte(chunk)); err != nil {
					zlog.Warnw("SSE write failed", "error", err)
					continue // 只打印日志，不中断循环
				}
				c.Writer.Flush()
				time.Sleep(mockStreamDelay) // 模拟延迟（更像真实 AI）
			}

			return
		}
	}

	metrics.IncOperation("ai", "cache_hit", "failure")
	zlog.Infow("AI 对话 (SSE) 缓存未命中", "question", req.Question)

	session, err := h.aiService.GetSession(c.Request.Context(), username)
	if err != nil {
		metrics.IncOperation("ai", "chat_sse", "failure: get session error")
		zlog.Warnw("获取会话失败", "detail", "无法获取或创建用户会话")
		// 记录错误，但是不终止，允许不借助对话历史
	}

	stream, err := h.aiService.StreamChat(c.Request.Context(), session, &req, modelID, userID)
	if err != nil {
		metrics.IncOperation("ai", "chat_sse", "failure: service error")
		zlog.Warnw("AI 对话请求失败", "detail", "AI 服务调用失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "result": false})

		return
	}

	var aiResp strings.Builder
	// 用于缓冲待发送的内容（提升性能）
	var buffer strings.Builder

	ticker := time.NewTicker(streamFlushTimeout)
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
		zlog.Errorw("AI 流式传输失败", "error", err, "question", req.Question)
		c.Writer.Write([]byte("\n[ERROR: 流式传输中断，请重试]"))
		c.Writer.Flush()

		return
	}

	go func() {
		ctx := context.Background()
		session.AddTurns(req.Question, aiResp.String())
		session.Trim(h.aiService.GetMaxSessionTurns())
		if saveErr := h.aiService.SaveSession(ctx, username, session); saveErr != nil {
			metrics.IncOperation("ai", "chat_sse", "failure: save session error")
			zlog.Warnw("AI 会话保存失败", "detail", "无法保存用户会话信息")
		}
	}()

	metrics.IncOperation("ai", "chat_sse", "success")
	zlog.Infow("AI 对话 (SSE) 请求成功")
}

func (h *AIHandler) AIChatWS(c *gin.Context) {
	start := time.Now()
	defer func() {
		metrics.ObserveOperationDuration("ai", "chat_ws", time.Since(start).Seconds())
	}()

	if c.Request.Header.Get("Upgrade") != "websocket" {
		metrics.IncOperation("ai", "chat_ws", "failure: upgrade required")
		zlog.Warnw("AI 对话请求失败", "detail", "升级为 WebSocket 协议失败")
		c.JSON(http.StatusBadRequest, gin.H{"msg": "upgrade required", "result": false})

		return
	}

	username := c.GetString("username")
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "unauthorized", "result": false})
		return
	}

	userID := c.GetInt64("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "缺少用户 ID"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zlog.Errorw("WebSocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	var writeMu sync.Mutex

	// 封装安全写函数，因为 WebSocket 的 conn 不是并发安全的
	safeWrite := func(msg model.WsMessage) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		conn.SetWriteDeadline(time.Now().Add(wsMaxWriteTimeout))

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

	defer func() {
		mu.Lock()
		if currentStream != nil {
			currentStream.cancel()
			select {
			case <-currentStream.done:
			case <-time.After(2 * time.Second):
			}
		}
		mu.Unlock()
	}()

	// 在 conn 成功 upgrade 后
	go func() {
		ticker := time.NewTicker(wsHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := safeWrite(model.WsMessage{Type: model.MsgTypePing}); err != nil {
					return
				}
			case <-c.Request.Context().Done(): // 可关联到连接生命周期
				return
			}
		}
	}()

	// 主消息循环
	for {
		// 设置读超时：wsReadTimeout 内必须发消息，否则断开
		conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				zlog.Warnw("WebSocket read message failed", "error", err)
			}

			return // 客户端断开，退出循环
		}

		var incoming model.WsMessage
		if err := json.Unmarshal(msgBytes, &incoming); err != nil {
			safeWrite(model.WsMessage{Type: model.MsgTypeError, Error: "invalid JSON format"})
			continue
		}

		switch incoming.Type {
		case model.MsgTypeChat:
			// 如果已有流在运行，先取消它
			mu.Lock()
			if currentStream != nil {
				currentStream.cancel()
				select {
				case <-currentStream.done:
				case <-time.After(2 * time.Second):
				}
			}

			// 处理聊天消息
			var req dto.AIChatRequest
			if err := json.Unmarshal([]byte(incoming.Data), &req); err != nil {
				mu.Unlock()
				safeWrite(model.WsMessage{Type: model.MsgTypeError, Error: "invalid JSON format"})

				continue
			}

			modelID := incoming.ModelID

			if modelID == defaultModelID {
				canCall, err := h.aiService.CheckAICallLimit(c.Request.Context(), userID)
				if err != nil {
					metrics.IncOperation("ai", "chat_ws", "failure: check call limit error")
					zlog.Warnw("检查 AI 调用限制失败", "detail", "无法检查用户调用限制")
					mu.Unlock()
					safeWrite(model.WsMessage{Type: model.MsgTypeError, Error: "检查调用限制失败"})

					continue
				}
				if !canCall {
					metrics.IncOperation("ai", "chat_ws", "failure: call limit exceeded")
					zlog.Warnw("AI 调用次数已达今日上限", "username", username)
					mu.Unlock()
					safeWrite(model.WsMessage{Type: model.MsgTypeError, Error: "AI 调用次数已达今日上限，请明日再试或使用自定义模型"})

					continue
				}
			}

			streamCtx, streamCancel := context.WithCancel(context.Background())
			doneChan := make(chan struct{})
			currentStream = &StreamControl{
				cancel: streamCancel,
				done:   doneChan,
			}
			mu.Unlock()

			qaCacheAnswer := h.aiService.CheckCache(req.Question)
			if qaCacheAnswer != "" {
				metrics.IncOperation("ai", "cache_hit", "success")
				mockStream, err := h.aiService.MockStreamChat(streamCtx, qaCacheAnswer)
				if err == nil {
					go func() {
						session, err := h.aiService.GetSession(streamCtx, username)
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

					go func() {
						defer close(doneChan)
						for mockStream.Next() {
							select {
							case <-streamCtx.Done():
								safeWrite(model.WsMessage{Type: model.MsgTypeStopped})
								return
							default:
								// 继续发送chuck
							}
							content := mockStream.Current()
							if err := safeWrite(model.WsMessage{Type: model.MsgTypeChunk, Data: content}); err != nil {
								zlog.Warnw("WebSocket write message failed", "error", err)
								return
							}
							time.Sleep(mockStreamDelay)
						}

						if err := mockStream.Err(); err != nil {
							safeWrite(model.WsMessage{Type: model.MsgTypeError, Error: err.Error()})
						} else {
							safeWrite(model.WsMessage{Type: model.MsgTypeDone})
						}
					}()

					continue
				}
			}

			metrics.IncOperation("ai", "cache_hit", "failure")
			go func() {
				defer close(doneChan)

				if modelID == defaultModelID {
					if err := h.aiService.IncrementAICallCount(context.Background(), userID); err != nil {
						metrics.IncOperation("ai", "chat_ws", "failure: increment call count error")
						zlog.Warnw("增加 AI 调用计数失败", "detail", "无法更新用户调用计数")
					}
				}

				session, err := h.aiService.GetSession(streamCtx, username)
				if err != nil {
					metrics.IncOperation("ai", "chat_ws", "failure: get session error")
					zlog.Errorw("Get session failed", "error", err)
					safeWrite(model.WsMessage{Type: model.MsgTypeError, Error: err.Error()})

					return
				}
				stream, err := h.aiService.StreamChat(streamCtx, session, &req, modelID, userID)
				if err != nil {
					metrics.IncOperation("ai", "chat_ws", "failure: stream chat error")
					zlog.Errorw("Stream chat failed", "error", err)
					safeWrite(model.WsMessage{Type: model.MsgTypeError, Error: err.Error()})

					return
				}

				var aiResp strings.Builder
				for stream.Next() {
					select {
					case <-streamCtx.Done():
						safeWrite(model.WsMessage{Type: model.MsgTypeStopped})
						return
					default:
						// 继续发送chuck
					}
					content := stream.Current().Choices[0].Delta.Content
					// 下面三个参数有些ai不会携带
					// fmt.Println("CompletionTokens: ", stream.Current().Usage.CompletionTokens)
					// fmt.Println("PromptTokens: ", stream.Current().Usage.PromptTokens)
					// fmt.Println("TotalTokens: ", stream.Current().Usage.TotalTokens)
					aiResp.WriteString(content)

					if err := safeWrite(model.WsMessage{Type: model.MsgTypeChunk, Data: content}); err != nil {
						zlog.Warnw("WebSocket write message failed", "error", err)
						return
					}
				}

				if err := stream.Err(); err != nil {
					safeWrite(model.WsMessage{Type: model.MsgTypeError, Error: err.Error()})
				} else {
					safeWrite(model.WsMessage{Type: model.MsgTypeDone})
				}

				go func() {
					session.AddTurns(req.Question, aiResp.String())
					session.Trim(h.aiService.GetMaxSessionTurns())
					if saveErr := h.aiService.SaveSession(context.Background(), username, session); saveErr != nil {
						metrics.IncOperation("ai", "chat_ws", "failure: save session error")
						zlog.Warnw("AI 会话保存失败", "detail", "无法保存用户会话信息")
					}
				}()
			}()
		case model.MsgTypeStop:
			mu.Lock()
			if currentStream != nil {
				currentStream.cancel()
				// 不等待done，立即响应
			}
			mu.Unlock()
			safeWrite(model.WsMessage{Type: model.MsgTypeStopped})
		case model.MsgTypePing:
			safeWrite(model.WsMessage{Type: model.MsgTypePong})
		default:
			safeWrite(model.WsMessage{Type: model.MsgTypeError, Error: "unknown message type"})
		}
	}
}

// GetAIConfig 获取用户的AI配置
func (h *AIHandler) GetAIConfig(c *gin.Context) {
	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "缺少用户ID"})
		return
	}

	// 获取AI配置
	config, err := h.aiService.GetAIConfig(c.Request.Context(), userID.(int64))
	if err != nil {
		zlog.Warnw("获取AI配置失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取AI配置失败", "result": false})

		return
	}

	c.JSON(http.StatusOK, config)
}

// AddCustomAIModel 添加自定义AI模型配置
func (h *AIHandler) AddCustomAIModel(c *gin.Context) {
	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "缺少用户ID"})
		return
	}

	// 绑定请求参数
	var req dto.CustomAIModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效", "result": false})
		return
	}

	// 验证AI客户端连接是否有效
	if err := h.aiService.ValidateClient(c.Request.Context(), req.APIKey, req.APIBaseURL); err != nil {
		zlog.Warnw("AI客户端验证失败", "error", err, "provider", req.Provider, "base_url", req.APIBaseURL)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "模型连接验证失败，请检查API密钥和Base URL是否正确",
			"detail": err.Error(),
			"result": false,
		})

		return
	}

	// 添加自定义模型
	_, err := h.aiService.AddCustomAIModel(c.Request.Context(), userID.(int64), &req)
	if err != nil {
		zlog.Warnw("添加自定义AI模型失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加自定义AI模型失败：" + err.Error(), "result": false})

		return
	}

	c.JSON(http.StatusOK, dto.CustomAIModelOperationResponse{
		Result: true,
		Msg:    "添加自定义AI模型成功",
	})
}

// GetCustomAIModels 获取用户的自定义AI模型配置列表
func (h *AIHandler) GetCustomAIModels(c *gin.Context) {
	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "缺少用户ID"})
		return
	}

	// 获取自定义模型列表
	models, err := h.aiService.GetCustomAIModels(c.Request.Context(), userID.(int64))
	if err != nil {
		zlog.Warnw("获取自定义AI模型列表失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取自定义AI模型列表失败", "result": false})

		return
	}

	// 转换为响应格式
	customModelResponses := make([]dto.CustomAIModelResponse, len(models))
	for i, model := range models {
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

	c.JSON(http.StatusOK, dto.CustomAIModelListResponse{
		Models: customModelResponses,
	})
}

// UpdateCustomAIModel 更新自定义AI模型配置
func (h *AIHandler) UpdateCustomAIModel(c *gin.Context) {
	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "缺少用户ID"})
		return
	}

	// 获取模型ID
	modelIDStr := c.Param("id")
	var modelID int64
	if _, err := fmt.Sscanf(modelIDStr, "%d", &modelID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的模型ID", "result": false})
		return
	}

	// 绑定请求参数
	var req dto.CustomAIModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效", "result": false})
		return
	}

	// 验证AI客户端连接是否有效
	if err := h.aiService.ValidateClient(c.Request.Context(), req.APIKey, req.APIBaseURL); err != nil {
		zlog.Warnw("AI客户端验证失败", "error", err, "provider", req.Provider, "base_url", req.APIBaseURL)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "模型连接验证失败，请检查API密钥和Base URL是否正确",
			"detail": err.Error(),
			"result": false,
		})

		return
	}

	// 更新自定义模型
	_, err := h.aiService.UpdateCustomAIModel(c.Request.Context(), userID.(int64), modelID, &req)
	if err != nil {
		zlog.Warnw("更新自定义AI模型失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新自定义AI模型失败", "result": false})

		return
	}

	c.JSON(http.StatusOK, dto.CustomAIModelOperationResponse{
		Result: true,
		Msg:    "更新自定义AI模型成功",
	})
}

// DeleteCustomAIModel 删除自定义AI模型配置
func (h *AIHandler) DeleteCustomAIModel(c *gin.Context) {
	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "缺少用户ID"})
		return
	}

	// 获取模型ID
	modelIDStr := c.Param("id")
	var modelID int64
	if _, err := fmt.Sscanf(modelIDStr, "%d", &modelID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的模型ID", "result": false})
		return
	}

	// 删除自定义模型
	if err := h.aiService.DeleteCustomAIModel(c.Request.Context(), userID.(int64), modelID); err != nil {
		zlog.Warnw("删除自定义AI模型失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除自定义AI模型失败", "result": false})

		return
	}

	c.JSON(http.StatusOK, dto.CustomAIModelOperationResponse{
		Result: true,
		Msg:    "删除自定义AI模型成功",
	})
}

// SwitchModel 切换模型
func (h *AIHandler) SwitchModel(c *gin.Context) {
	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "缺少用户ID"})
		return
	}

	// 绑定请求参数
	var req dto.SwitchModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效", "result": false})
		return
	}

	// 切换模型
	if err := h.aiService.UpdateUserCurrentModel(c.Request.Context(), userID.(int64), req.ModelID); err != nil {
		zlog.Warnw("切换模型失败", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "切换模型失败", "result": false})

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "切换模型成功",
	})
}
