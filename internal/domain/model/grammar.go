package model

// Symbol 表示一个符号，可以是终结符、非终结符或者自动机所识别的一个符号
type Symbol string

const Epsilon Symbol = "ε" // 定义ε作为特殊输入符号，表示空转移符号

/* 文法 */
// Production 规定了一个产生式
type Production struct {
	Left  []Symbol `json:"left"`  // 左部，通常是单个非终结符，但也可以是多个符号
	Right []Symbol `json:"right"` // 右部，可以包含多个符号
}

// Grammar 表示整个文法
type Grammar struct {
	StartSymbol  Symbol       `json:"startSymbol"`  // 起始符号
	Terminals    []Symbol     `json:"terminals"`    // 终结符集合
	NonTerminals []Symbol     `json:"nonTerminals"` // 非终结符集合
	Productions  []Production `json:"productions"`  // 产生式集合
}
