# AI 服务

## 模块定位

AI 服务是 FormalLangLab 项目的智能问答模块，提供基于大语言模型的形式语言与自动机理论智能辅导功能。该模块通过上下文感知的提示工程（context-aware prompting）与教育导向的角色设定，使 AI 助手能够提供具有对话连贯性与教学引导性的解释，实现"近似一对一辅导"的交互体验。

## 功能特性

- **实时 AI 聊天**：基于 WebSocket 的双向通信，支持流式响应
- **SSE 流式输出**：基于 Server-Sent Events 的单向流式响应
- **领域专业知识**：支持自动机、文法、正则表达式等专业领域问答
- **会话管理**：基于 Redis 的会话上下文管理和持久化
- **知识库缓存**：支持 QA 知识库缓存，提升响应速度
- **用户自定义模型**：支持用户配置个性化的 AI 模型参数
- **多页面上下文**：根据不同页面类型（general/grammar/regex/automaton）提供针对性回答

## 技术栈

| 类别 | 技术 |
|------|------|
| 开发语言 | Go 1.23+ |
| Web 框架 | Gin |
| 缓存中间件 | Redis |
| 消息队列 | Kafka |
| 大模型 API | DASHSCOPE（通义千问） |
| 数据库 | PostgreSQL |
| 日志系统 | 结构化日志（zlog） |
| 配置管理 | YAML 配置文件 |
| 监控指标 | Prometheus + Grafana |

## 项目结构

```
backend/ai/
├── cmd/                        # 程序入口
│   └── main.go                 # 主服务入口
├── configs/                    # 配置管理
│   ├── config.go               # 配置结构定义与加载
│   └── config.yaml.example     # 配置文件示例
├── internal/                   # 核心业务代码
│   ├── core/                   # 核心逻辑
│   │   ├── page.go             # 页面类型定义
│   │   └── validators.go       # 自定义验证器
│   ├── domain/                 # 领域模型
│   │   ├── dto/                # 数据传输对象
│   │   │   └── ai_dto.go       # AI 相关 DTO
│   │   └── model/              # 领域实体模型
│   │       ├── ai.go           # AI 模型
│   │       ├── automaton.go    # 自动机模型
│   │       ├── grammar.go      # 文法模型
│   │       ├── regex.go        # 正则表达式模型
│   │       └── qacache.go      # QA 缓存模型
│   ├── handler/                # HTTP 接口层
│   │   ├── ai.go               # AI 处理器
│   │   └── router.go           # 路由定义
│   ├── service/                # 业务逻辑层
│   │   ├── ai_service.go       # AI 服务实现
│   │   ├── openai_client.go    # OpenAI 客户端封装
│   │   └── qacache.go          # QA 缓存服务
│   ├── repository/             # 数据访问层
│   │   ├── ai_repo.go          # AI 数据访问
│   │   ├── user_ai_repo.go     # 用户 AI 模型数据访问
│   │   ├── qacache.go          # QA 缓存数据访问
│   │   ├── file_watcher.go     # 文件监听器
│   │   └── init.go             # 存储库初始化
│   ├── middleware/             # 中间件
│   │   ├── cors/               # 跨域中间件
│   │   ├── jwt/                # JWT 认证中间件
│   │   ├── metrics/            # 指标采集中间件
│   │   ├── rate/               # 速率限制中间件
│   │   └── requestid/          # 请求 ID 中间件
│   ├── server/                 # 服务器封装
│   │   └── server.go           # HTTP 服务器配置
│   └── utils/                  # 工具函数
│       └── crypto.go           # 加密工具
├── asset/                      # 静态资源
│   └── qacache/                # QA 知识库缓存文件
├── migrations/                 # 数据库迁移脚本
├── pkg/                        # 公共包
│   └── binding/                # 参数绑定与验证
├── logs/                       # 日志文件目录
├── .gitignore                  # Git 忽略配置
├── .golangci.yml               # Go 代码检查配置
├── Dockerfile                  # Docker 镜像构建文件
├── Makefile                    # Make 命令配置
├── go.mod                      # Go 模块依赖
└── go.sum                      # Go 依赖校验文件
```

## 中间件

| 中间件 | 文件路径 | 功能描述 |
|--------|---------|---------|
| RequestID | `middleware/requestid/requestid.go` | 为每个请求生成唯一 ID，便于日志追踪 |
| CORS | `middleware/cors/cors.go` | 处理跨域请求，支持预检请求 |
| JWT | `middleware/jwt/jwt.go` | JWT Token 认证，从请求头中提取用户信息 |
| UserRateLimit | `middleware/rate/rate.go` | 用户级速率限制，防止恶意请求 |
| Metrics | `middleware/metrics/metrics.go` | Prometheus 指标采集，监控请求延迟和 QPS |

## 核心服务

### AI 服务（AI Service）

**文件**：`internal/service/ai_service.go`

**功能**：
- 处理用户 AI 聊天请求
- 构建上下文感知的提示词
- 调用大模型 API 生成回答
- 管理会话历史和上下文
- 支持流式响应输出

### QA 缓存服务（QA Cache Service）

**文件**：`internal/service/qacache.go`

**功能**：
- 加载和管理 QA 知识库缓存
- 文件监听和热更新
- 基于 RAG 的检索增强生成
- 缓存命中判断和检索

### OpenAI 客户端（OpenAI Client）

**文件**：`internal/service/openai_client.go`

**功能**：
- 封装 DASHSCOPE API 调用
- 管理 API 密钥和配置
- 处理流式响应
- 错误处理和重试机制

## 数据访问层

### AI 存储库（AI Repository）

**文件**：`internal/repository/ai_repo.go`

**功能**：
- 用户 AI 会话的持久化
- 会话历史记录管理
- 自定义 AI 模型配置存储

### QA 缓存存储库（QA Cache Repository）

**文件**：`internal/repository/qacache.go`

**功能**：
- QA 知识库文件加载
- 缓存数据检索
- 文件变更监听

## 领域模型

### 数据传输对象（DTO）

**文件**：`internal/domain/dto/ai_dto.go`

**主要 DTO**：
- `ChatRequest`：聊天请求体
- `ChatResponse`：聊天响应体
- `SaveSessionRequest`：保存会话请求体
- `AIModelConfig`：AI 模型配置

### 领域实体（Model）

**文件**：`internal/domain/model/`

**主要模型**：
- `AIChatSession`：AI 聊天会话
- `UserAIModel`：用户自定义 AI 模型
- `QACache`：QA 知识库缓存项

## API 接口

### WebSocket 聊天

- **路由**：`GET /gdesign/ai/ws`
- **认证**：JWT Token
- **功能**：建立 WebSocket 连接，进行实时双向聊天

### SSE 聊天

- **路由**：`GET /gdesign/ai/chat`
- **认证**：JWT Token
- **功能**：建立 SSE 连接，接收流式响应

### 保存会话

- **路由**：`POST /gdesign/ai/save-session`
- **认证**：JWT Token
- **功能**：保存当前聊天会话到数据库

### 健康检查

- **路由**：`GET /api/health`
- **功能**：服务健康状态检查

## 配置说明

**文件**：`configs/config.yaml.example`

**主要配置项**：
- 服务端口配置
- Redis 连接配置
- PostgreSQL 数据库配置
- DASHSCOPE API 密钥配置
- 日志配置
- Kafka 配置
- JWT 配置

## 核心算法

### RAG（检索增强生成）

AI 服务采用 RAG 技术，结合知识库缓存和大模型能力：
1. 从用户问题中提取关键信息
2. 在 QA 知识库中检索相关内容
3. 构建包含检索结果的提示词
4. 调用大模型生成回答
5. 流式返回给用户

### 上下文管理

- 基于 Redis 存储会话上下文
- 维护最近 N 轮对话历史
- 根据页面类型自动调整上下文内容
- 支持会话持久化和恢复

## Docker 与 Makefile 使用说明

### Dockerfile 说明

Dockerfile 采用多阶段构建，分为构建阶段和运行阶段：

**构建阶段**：
- 使用 Go 1.24 Alpine 镜像作为构建环境
- 配置国内 Go 代理镜像源加速依赖下载
- 先下载依赖再复制源代码，优化 Docker 缓存层
- 构建静态二进制文件（CGO_ENABLED=0）

**运行阶段**：
- 使用 Alpine 3.20 精简镜像
- 创建非 root 用户（aiuser）运行应用，提升安全性
- 配置 Asia/Shanghai 时区
- 复制二进制文件和配置文件
- 暴露服务端口（默认 8081）

### Makefile 命令

| 命令 | 说明 | 示例 |
|------|------|------|
| `make lint` | 运行代码质量检查 | `make lint` |
| `make lint-fix` | 运行代码质量检查并自动修复 | `make lint-fix` |
| `make build` | 构建并推送 Docker 镜像 | `make build` |
| `make build T=false` | 仅构建镜像，不推送 | `make build T=false` |
| `make deploy` | 构建镜像并部署服务 | `make deploy` |

**快速开始**：
```bash
# 1. 代码质量检查
make lint

# 2. 构建镜像（本地测试，不推送）
make build T=false

# 3. 构建并推送镜像
make build

# 4. 部署服务
make deploy
```
