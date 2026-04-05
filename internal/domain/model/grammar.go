package model

// Symbol 表示一个符号，可以是终结符、非终结符或者自动机所识别的一个符号
type Symbol string

const Epsilon Symbol = "ε" // 定义ε作为特殊输入符号，表示空转移符号

type GrammarType int

const (
	InvalidGrammar          GrammarType = iota - 1 // iota=0 → 0-1 = -1
	PhraseStructureGrammar                         // iota=1 → 1-1 = 0
	ContextSensitiveGrammar                        // iota=2 → 2-1 = 1
	ContextFreeGrammar                             // iota=3 → 3-1 = 2
	RegularGrammar                                 // iota=4 → 4-1 = 3
)

var GrammarTypeNameMap = map[GrammarType]string{
	RegularGrammar:          "3型文法（正则文法）",
	ContextFreeGrammar:      "2型文法（上下文无关文法）",
	ContextSensitiveGrammar: "1型文法（上下文有关文法）",
	PhraseStructureGrammar:  "0型文法（短语结构文法）",
	InvalidGrammar:          "无效文法",
}

// NormalizeSymbol 工具函数：将用户输入映射为标准 ε
func NormalizeSymbol(s string) Symbol {
	switch s {
	case "ε", "epsilon", "e", "E", "", "λ", "eps":
		return Epsilon
	default:
		return Symbol(s)
	}
}

/* 文法 */
// Production 规定了一个产生式
type Production struct {
	Left  []Symbol `json:"left"`  // 左部，通常是单个非终结符，但也可以是多个符号
	Right []Symbol `json:"right"` // 右部，可以包含多个符号
}

// Grammar 表示整个文法
type Grammar struct {
	StartSymbol  Symbol       `json:"startSymbol"`                           // 起始符号
	Terminals    []Symbol     `json:"terminals"`                             // 终结符集合
	NonTerminals []Symbol     `json:"nonTerminals"`                          // 非终结符集合
	Productions  []Production `json:"productions"`                           // 产生式集合
	GrammarType  GrammarType  `json:"grammarType" default:"PhraseStructure"` // 文法类型
}
