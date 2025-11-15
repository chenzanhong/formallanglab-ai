# AI 服务

这是GDesign项目的独立AI服务模块，提供形式语言与自动机理论的智能问答功能。

> 注意：本服务是从主项目中拆分出来的独立模块，负责处理所有AI相关的功能。

## 功能特性

- 基于WebSocket的实时AI聊天
- 基于SSE的流式响应
- 支持自动机、文法、正则表达式等专业领域问答
- 会话管理和持久化（基于Redis）
- 知识库缓存支持

## 环境要求

- Go 1.23+
- Redis 6.0+
- OpenAI API 密钥（DASHSCOPE API）

## 快速开始

### 1. 配置环境变量

复制环境变量模板并根据实际情况修改：

```bash
cp .env.example .env
```

### 2. 安装依赖

```bash
go mod tidy
```

### 3. 启动服务

```bash
go run cmd/main.go
```

服务默认在 `8081` 端口启动。

## API 接口

### 健康检查

- **GET /api/health**
  - 响应：`{"status": "ok", "service": "ai-service"}`

### WebSocket 聊天

- **GET /gdesign/ai/ws**
  - 需要JWT认证（username从token中提取）
  - 支持的页面类型：general/grammar/regex/automaton
  - 消息格式：JSON

### SSE 聊天

- **GET /gdesign/ai/chat**
  - 需要JWT认证（username从token中提取）
  - 请求体：`{"question": "...", "page": "...", "automaton": {...}, "grammar": {...}, "regex": "..."}`
  - 响应：SSE 流式响应

### 会话保存

- **POST /gdesign/ai/save-session**
  - 需要JWT认证
  - 请求体：`{"title": "会话标题", "recent_turns": [...], "page": "页面类型"}`

## 项目结构

- `cmd/` - 入口文件目录
  - `main.go` - 主程序入口
- `configs/` - 配置管理
  - `config.go` - 配置加载逻辑
  - `config.yaml` - 配置文件
- `internal/api/` - API处理层
  - `ai.go` - AI相关API处理器
- `internal/service/` - 业务逻辑层
  - `ai_service.go` - AI服务实现
  - `openai_client.go` - OpenAI客户端
  - `qa_cache.go` - QA缓存服务
- `internal/repository/` - 数据访问层
  - `init.go` - 存储库初始化
  - `ai_repository.go` - AI数据访问
- `internal/domain/` - 领域模型
  - `dto/` - 数据传输对象
  - `model/` - 领域实体模型
- `pkg/` - 通用工具包
  - `binding/` - 数据绑定和验证
- `logs/` - 日志管理
- `.env.example` - 环境变量示例文件

## 注意事项

1. 确保Redis服务正常运行
2. 配置正确的OpenAI/DASHSCOPE API密钥
3. 服务默认使用8081端口，可通过环境变量SERVER_PORT修改
4. 项目已配置CORS中间件，允许跨域访问
5. 服务提供JWT认证机制，确保安全性