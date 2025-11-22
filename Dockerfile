# 第一阶段：构建阶段
FROM golang:1.24-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装依赖管理工具 - 减少镜像层
RUN apk --no-cache add git gcc musl-dev && \
    go version && \
    echo "构建环境准备完成"

# 复制go.mod和go.sum文件
COPY go.mod go.sum ./

# 设置国内Go代理镜像源，加速依赖下载
ENV GOPROXY=https://goproxy.cn,direct

# 下载依赖（使用go mod download替代go mod tidy以提高构建速度）
RUN go mod download

# 缓存层优化：先下载依赖，然后再复制源代码，这样修改代码不会重新下载依赖

# 复制所有源代码
COPY . .

# 构建应用，设置CGO_ENABLED=0以创建静态二进制文件
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ai-server cmd/main.go

# 第二阶段：运行阶段
FROM alpine:3.20

# 添加安全标签
LABEL maintainer="GDesign Team"
LABEL version="1.0"
LABEL description="AI Service for GDesign Project"

# 设置工作目录
WORKDIR /app

# 合并RUN指令，减少镜像层，同时创建非root用户
RUN apk --no-cache add ca-certificates tzdata wget && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    addgroup -g 1000 aiuser && \
    adduser -u 1000 -G aiuser -h /app -D aiuser && \
    mkdir -p /app/logs /app/configs /app/asset/qacache && \
    chown -R aiuser:aiuser /app

# 切换到非root用户运行应用
USER aiuser

# 从构建阶段复制二进制文件
COPY --from=builder /app/ai-server /app/

# 复制配置文件
COPY configs/config.yaml /app/configs/

# 创建环境变量示例文件 - 使用COPY替代RUN echo以提高可读性
COPY --chown=aiuser:aiuser .env.example /app/

# 暴露服务端口（使用环境变量允许动态配置）
EXPOSE ${SERVER_PORT:-8082}
EXPOSE ${PPROF_PORT:-6060}

# 设置健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:${SERVER_PORT:-8082}/api/health || exit 1

# 运行应用（使用exec形式的CMD以确保信号能正确传递）
CMD ["/app/ai-server"]

# ======================= 使用说明 =======================
# 1. 构建镜像：
#    docker build -t gdesign-ai .
#
# 2. 准备环境：
#    - 创建.env文件（从.env.example复制并填写实际密钥）
#    - 确保Redis服务正在运行
#
# 3. 运行容器（方式1：使用环境变量传递敏感信息）：
#    docker run -d \
#      --name gdesign-ai \
#      -p 8082:8082 \
#      -e DASHSCOPE_API_KEY=your_actual_key \
#      -e REDIS_HOST=host.docker.internal \
#      -e JWT_KEY=your_actual_jwt_key \
#      gdesign-ai
#
# 4. 运行容器（方式2：使用卷挂载配置文件）：
#    docker run -d \
#      --name gdesign-ai \
#      -p 8082:8082 \
#      -v $(pwd)/.env:/app/.env \
#      -v $(pwd)/configs:/app/configs \
#      -v $(pwd)/logs:/app/logs \
#      -v $(pwd)/asset/qacache:/app/asset/qacache \
#      gdesign-ai
#
# 注意：敏感信息（如API密钥）应通过环境变量或安全的卷挂载方式提供，
# 避免直接硬编码在镜像中。生产环境建议使用Docker Secrets或环境变量。