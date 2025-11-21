package model

import (
	"ai/internal/core"
)

type AISession struct {
	Page        core.PageType `json:"page"`
	LastActive  int64         `json:"last_active"`            // 最后活跃时间
	RecentTurns []QAPair      `json:"recent_turns,omitempty"` // 最近的对话记录
	Summary     string        `json:"summary,omitempty"`      // 会话总结
	Token       int           `json:"token"`                  // 当前会话的token数
}

type QAPair struct {
	User string `json:"user"` // 用户输入
	AI   string `json:"ai"`   // AI 回复
}

// ========== WebSocket ===========
type WsMessageType string

const (
	MsgTypeChat    WsMessageType = "chat"    // 客户端发送聊天消息
	MsgTypeStop    WsMessageType = "stop"    // 客户端请求停止生成
	MsgTypePing    WsMessageType = "ping"    // 客户端发送心跳包
	MsgTypePong    WsMessageType = "pong"    // 服务器响应心跳包
	MsgTypeDone    WsMessageType = "done"    // 服务器发送完成消息（流式结束）
	MsgTypeChunk   WsMessageType = "chunk"   // 服务器发送流式消息块
	MsgTypeError   WsMessageType = "error"   // 服务器发送错误消息
	MsgTypeStopped WsMessageType = "stopped" // 通知客户端生成已停止
)

type WsMessage struct {
	Type  WsMessageType `json:"type"` //  "chat", "stop", "chunk", "done", "error", "stopped"
	Data  string        `json:"data,omitempty"`
	Error string        `json:"error,omitempty"`
}

func (s *AISession) Trim(maxTurns int) {
	if len(s.RecentTurns) > maxTurns {
		s.RecentTurns = s.RecentTurns[len(s.RecentTurns)-maxTurns:]
	}
}

func (s *AISession) TrimWithToken(maxToken int) {
	// 优先保留最新的记录
}

func (s *AISession) AddTurns(user, ai string) {
	s.RecentTurns = append(s.RecentTurns, QAPair{
		User: user,
		AI:   ai,
	})
}
