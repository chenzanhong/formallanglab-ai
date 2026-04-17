<!-- aliases: ["什么是ε-闭包"] -->
**ε-闭包（epsilon-closure）** 是在处理包含 **ε 转移（空转移）** 的 **非确定有限自动机（NFA-ε）** 时的一个关键概念。

---

### 📌 定义

对于 NFA 中的某个状态 \( q \)，其 **ε-闭包**（记作 \( \varepsilon\text{-closure}(q) \)）是指：

> 从状态 \( q \) 出发，**仅通过零个或多个 ε 转移**（不消耗任何输入符号）能够到达的所有状态的集合（包括 \( q \) 本身）。

更一般地，对于一个状态集合 \( S \subseteq Q \)，  
\[
\varepsilon\text{-closure}(S) = \bigcup_{q \in S} \varepsilon\text{-closure}(q)
\]

---

### 🔍 直观理解

- ε 转移表示“自动跳转”，不需要读取输入字符。
- 所以，当你处于某个状态时，你实际上可能“同时”处于它所有 ε 可达的状态。
- ε-闭包就是把这些“隐含可达”的状态全部找出来。

---

### 🧪 示例

假设 NFA 的状态和 ε 转移如下：
- 状态：{0, 1, 2}
- ε 转移：
  - \( 0 \xrightarrow{\varepsilon} 1 \)
  - \( 1 \xrightarrow{\varepsilon} 2 \)

那么：
- \( \varepsilon\text{-closure}(0) = \{0, 1, 2\} \)  
  （从 0 出发，先到 1，再到 2，都不需要输入）
- \( \varepsilon\text{-closure}(1) = \{1, 2\} \)
- \( \varepsilon\text{-closure}(2) = \{2\} \)（没有从 2 出发的 ε 转移）

如果有一个状态集合 \( S = \{0, 2\} \)，则：
- \( \varepsilon\text{-closure}(S) = \varepsilon\text{-closure}(0) \cup \varepsilon\text{-closure}(2) = \{0,1,2\} \cup \{2\} = \{0,1,2\} \)

---

### 🛠 在 NFA → DFA 转换中的作用

在使用**子集构造法**将 NFA-ε 转换为 DFA 时，每一步都必须考虑 ε-闭包：

1. **DFA 的初始状态** = \( \varepsilon\text{-closure}(\{q_0\}) \)
2. 对于当前 DFA 状态 \( S \)（即 NFA 状态集合）和输入符号 \( a \)：
   - 先计算 **move(S, a)** = 所有从 \( S \) 中状态读入 \( a \) 能直接到达的状态
   - 再计算 **\( \varepsilon\text{-closure}(\text{move}(S, a)) \)**，作为下一个 DFA 状态

这样才能确保所有“隐式可达”的状态都被包含进来。

---

### ✅ 小结

| 概念 | 含义 |
|------|------|
| ε 转移 | 不消耗输入符号的状态跳转 |
| ε-闭包(q) | 从 q 出发仅通过 ε 转移能到达的所有状态（含 q 自身） |
| 用途 | 处理 NFA-ε，构建等价 DFA，模拟“同时处于多个状态”的行为 |

如果你有具体的 NFA 图或转移表，我可以帮你一步步计算 ε-闭包！