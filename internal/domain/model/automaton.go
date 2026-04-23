package model

/* 自动机 */

// State 表示自动机中的一个状态
type State string

// Transition 表示一个状态转移规则
type Transition struct {
	FromState State   `json:"fromState"` // 起始状态
	Input     Symbol  `json:"input"`     // 输入符号
	ToStates  []State `json:"toStates"`  // 目标状态（对于 NFA 可以有多个，但是目前大部分还是分开来的，不合并相同 FromState+Input 的产生式）
}

// Automaton 基础自动机结构
type Automaton struct {
	States          []State      `json:"states"`          // 状态集合
	Alphabet        []Symbol     `json:"alphabet"`        // 符号表
	Transitions     []Transition `json:"transitions"`     // 状态转移规则集合
	InitialState    State        `json:"initialState"`    // 初始状态
	AcceptingStates []State      `json:"acceptingStates"` // 接受状态集合
}
