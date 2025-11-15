// internal/core/page.go
package core

const (
	PageRegex     = "regex"
	PageGrammar   = "grammar"
	PageAutomaton = "automaton"
	PageLearn     = "learn"
	PageHome      = "home"
)

var ValidPages = map[string]bool{
	PageRegex:     true,
	PageGrammar:   true,
	PageAutomaton: true,
	PageLearn:     true,
	PageHome:      true,
}

type PageType string

// 可选：提供校验方法（非 validator 专用）
func (p PageType) IsValid() bool {
	return ValidPages[string(p)]
}