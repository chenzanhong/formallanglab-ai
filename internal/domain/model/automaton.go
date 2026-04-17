package model

/* 自动机 */
// Symbol 表示自动机中的输入符号
// type Symbol string

// State 表示自动机中的一个状态
type State string

// const Epsilon Symbol = "ε" // 定义ε作为特殊输入符号，表示空转移符号

// Transition 表示一个状态转移规则
type Transition struct {
	FromState State   `json:"fromState"` // 起始状态
	Input     Symbol  `json:"input"`     // 输入符号
	ToStates  []State `json:"toStates"`  // 目标状态（对于 NFA 可以有多个，但是目前大部分还是分开来的，不合并相同 FromState+Input 的产生式）
}

type AutomatonType int

const (
	DFA        AutomatonType = 0 // 确定有限自动机
	NFA        AutomatonType = 1 // 非确定有限自动机
	EpsilonNFA AutomatonType = 2 // 非确定有限自动机（允许 ε-转移）
)

// Automaton 基础自动机结构
type Automaton struct {
	States          []State                      `json:"states"`          // 状态集合
	Alphabet        []Symbol                     `json:"alphabet"`        // 符号表
	Transitions     []Transition                 `json:"transitions"`     // 状态转移规则集合
	InitialState    State                        `json:"initialState"`    // 初始状态
	AcceptingStates []State                      `json:"acceptingStates"` // 接受状态集合
	Type            AutomatonType                `json:"type"`            // 是否为 DFA，否则为 NFA
	TransMap        map[State]map[Symbol][]State `json:"-"`               // Map 存储状态转移规则，识别字符串时效率高
}
