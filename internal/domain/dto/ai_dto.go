package dto

import (
	"ai/internal/domain/model"
	"ai/internal/core"
)

type AIChatRequest struct {
	Question  string           `json:"question" binding:"required"`
	Page      core.PageType `json:"page" binding:"pageValid"`
	Automaton *model.Automaton `json:"automaton,omitempty"` // ← 指针
	Grammar   *model.Grammar   `json:"grammar,omitempty"`
	Regex     *model.Regex     `json:"regex,omitempty"`
}
