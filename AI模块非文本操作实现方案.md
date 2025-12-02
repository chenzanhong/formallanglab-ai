# AI模块非文本操作能力实现方案

## 1. 背景与需求分析

### 1.1 当前AI模块能力边界

基于代码分析，当前AI模块主要实现了以下功能：
- 通过WebSocket和SSE两种方式进行文本对话交互
- 支持上下文感知（自动机、文法结构）
- 提供流式响应
- 集成了阿里云Qwen API
- 具备基本的问题缓存功能

但目前仅支持文本问答，不具备以下能力：
- 自动机/文法的错误诊断与修复
- 代码/文法的自动补全
- 非文本操作（如直接修改用户的自动机状态）
- 交互式调试和验证

### 1.2 需求概述

为AI模块增加以下非文本操作能力：
- **错误修复**：自动检测并修复用户创建的自动机、文法中的错误
- **自动补全**：在用户编辑自动机/文法过程中提供智能补全建议
- **交互式操作**：AI能够直接修改系统状态，而不仅是提供文本建议
- **可视化辅助**：提供自动机/文法的可视化改进建议

## 2. 技术方案设计

### 2.1 整体架构

整体采用分层架构设计：

1. **接口层**：扩展现有API，增加非文本操作相关端点
2. **业务逻辑层**：新增专用服务处理非文本操作请求
3. **形式化分析层**：负责自动机/文法的语法检查、语义分析
4. **LLM接口层**：增强与大语言模型的交互，增加结构化输出能力
5. **执行引擎层**：负责执行AI生成的修改操作

### 2.2 核心技术栈

| 技术/组件 | 用途 | 选型理由 |
|---------|------|--------|
| Go | 后端服务开发 | 高性能、并发能力强，与现有系统一致 |
| TypeScript | 前端实现 | 类型安全，与现有React前端兼容 |
| Qwen API | 基础LLM能力 | 已在系统中集成，支持结构化输出 |
| 向量数据库 | 上下文检索 | 增强问题理解和方案生成准确性 |
| 形式化验证引擎 | 语法检查与验证 | 确保AI生成的修复方案正确性 |
| WebSocket | 实时交互 | 已在系统中实现，支持双向通信 |

### 2.3 功能模块详细设计

#### 2.3.1 错误诊断与修复模块

**核心功能**：
- 自动检测自动机/文法定义中的错误
- 提供具体错误位置和原因分析
- 生成修复方案并支持一键应用

**实现方案**：

```go
// internal/service/ai_s/error_fixer.go
type ErrorFixer interface {
    // 诊断自动机错误
    DiagnoseAutomaton(automaton *model.Automaton) ([]*model.Diagnostic, error)
    // 生成修复方案
    GenerateFix(automaton *model.Automaton, diagnostics []*model.Diagnostic) (*model.FixSolution, error)
    // 应用修复
    ApplyFix(automaton *model.Automaton, fix *model.FixSolution) (*model.Automaton, error)
}
```

#### 2.3.2 自动补全模块

**核心功能**：
- 基于上下文提供智能补全建议
- 支持状态、转移、产生式等多种补全类型
- 提供补全预览和接受功能

**实现方案**：

```typescript
// frontend/src/services/autoCompleteService.ts
export interface AutoCompleteItem {
  type: 'state' | 'transition' | 'production' | 'alphabet';
  value: string;
  description: string;
  priority: number;
}

export async function getAutoCompleteSuggestions(
  context: AutoCompleteContext,
  prefix: string
): Promise<AutoCompleteItem[]> {
  // 调用后端API获取补全建议
  // 返回结构化的补全项
}
```

#### 2.3.3 执行引擎模块

**核心功能**：
- 安全执行AI生成的修改操作
- 维护操作历史，支持撤销/重做
- 提供操作预览功能

**实现方案**：

```go
// internal/service/ai_s/execution_engine.go
type ExecutionEngine interface {
    // 执行修改操作
    ExecuteOperation(ctx context.Context, op *model.AIOperation) (*model.OperationResult, error)
    // 预览操作效果
    PreviewOperation(ctx context.Context, op *model.AIOperation) (*model.PreviewResult, error)
    // 撤销操作
    UndoOperation(ctx context.Context, operationID string) error
    // 重做操作
    RedoOperation(ctx context.Context, operationID string) error
}
```

#### 2.3.4 形式化分析模块

**核心功能**：
- 自动机/文法的语法和语义检查
- 等价性验证
- 语言包含关系分析

**实现方案**：

```go
// internal/service/formal/formal_analyzer.go
type FormalAnalyzer interface {
    // 验证自动机正确性
    ValidateAutomaton(automaton *model.Automaton) ([]*model.ValidationError, error)
    // 检查自动机等价性
    CheckEquivalence(automaton1, automaton2 *model.Automaton) (bool, error)
    // 分析语言包含关系
    CheckLanguageInclusion(superset, subset *model.Automaton) (bool, error)
}
```

### 2.4 API设计

#### 2.4.1 后端API扩展

```go
// internal/api/ai_extended.go
func (h *AIHandler) DiagnoseAndFix(c *gin.Context) {
    // 1. 接收自动机/文法定义
    // 2. 执行诊断
    // 3. 生成修复方案
    // 4. 返回诊断结果和修复建议
}

func (h *AIHandler) ApplyFix(c *gin.Context) {
    // 1. 接收修复方案
    // 2. 应用修复
    // 3. 返回修复后的结构
}

func (h *AIHandler) GetAutoComplete(c *gin.Context) {
    // 1. 接收上下文和前缀
    // 2. 生成补全建议
    // 3. 返回结构化补全项
}
```

#### 2.4.2 前端接口

```typescript
// frontend/src/api/aiExtended.ts
export async function diagnoseAndFixAutomaton(automaton: Automaton) {
  // 调用后端诊断API
}

export async function applyFixSolution(solution: FixSolution) {
  // 调用后端应用修复API
}

export async function getAutoCompleteSuggestions(context: any, prefix: string) {
  // 调用后端自动补全API
}
```

### 2.5 数据流设计

**错误修复数据流**：
1. 用户提交自动机/文法定义
2. 后端进行形式化验证
3. 检测到错误后，构建错误上下文
4. 调用LLM生成修复方案
5. 形式化引擎验证修复方案有效性
6. 返回修复建议给前端
7. 用户确认后，应用修复并更新状态

**自动补全数据流**：
1. 用户输入触发补全请求
2. 前端收集当前编辑上下文
3. 后端分析上下文，调用LLM生成补全建议
4. 对补全建议进行排序和过滤
5. 返回结构化补全项给前端
6. 用户选择后应用补全

## 3. 实现计划

### 3.1 分阶段实现

**第一阶段：基础架构**
- 扩展数据模型，支持诊断和修复操作
- 实现形式化分析基础功能
- 扩展API接口

**第二阶段：核心功能**
- 实现错误诊断与修复功能
- 开发自动补全模块
- 构建执行引擎

**第三阶段：用户体验**
- 优化前端交互
- 实现操作预览和历史记录
- 性能优化

### 3.2 技术挑战与解决方案

| 挑战 | 解决方案 |
|-----|--------|
| 修复方案正确性 | 结合形式化验证和LLM生成，双重保障 |
| 执行安全性 | 沙箱环境执行，限制操作范围 |
| 实时响应性能 | 缓存常用诊断结果，预计算补全项 |
| 用户体验 | 渐进式UI设计，提供操作预览和撤销功能 |

## 4. 风险评估

| 风险 | 影响 | 缓解措施 |
|-----|------|--------|
| LLM生成不正确的修复方案 | 高 | 增加形式化验证环节，对修复方案进行正确性检查 |
| 系统复杂性增加 | 中 | 模块化设计，保持良好的代码组织和文档 |
| 性能开销 | 中 | 缓存策略，异步处理，批处理优化 |
| 用户接受度 | 中 | 提供明确的操作预览和撤销功能，增强透明度 |

## 5. 总结

通过上述设计，我们可以为AI模块增加强大的非文本操作能力，使用户能够获得更智能的辅助体验。该方案充分利用了现有的技术栈和架构，通过合理的扩展和优化，实现错误修复、自动补全等高级功能，同时确保系统的安全性和性能。

实施该方案后，FormalLangLab系统将从单纯的文本问答工具升级为真正的智能辅助系统，能够主动识别和解决问题，大幅提升用户学习效率。