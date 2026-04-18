# 第一阶段：构建阶段
FROM crpi-tcnuencv1iecgx03.cn-hangzhou.personal.cr.aliyuncs.com/chenzh2004/golang:1.24-alpine AS builder

# 设置工作目录
WORKDIR /app

# # 安装依赖管理工具 - 减少镜像层
# RUN apk --no-cache add git gcc musl-dev && \
#     go version && \
#     echo "构建环境准备完成"

# 复制go.mod和go.sum文件
COPY go.mod go.sum ./

# 设置国内Go代理镜像源，加速依赖下载
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=off

# 下载依赖（使用go mod download替代go mod tidy以提高构建速度）
RUN go mod download

# 缓存层优化：先下载依赖，然后再复制源代码，这样修改代码不会重新下载依赖

# 复制所有源代码
COPY . .

# 构建应用，设置CGO_ENABLED=0以创建静态二进制文件
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o ai-server cmd/main.go

# 第二阶段：运行阶段
# 使用阿里云镜像源加速镜像拉取
FROM crpi-tcnuencv1iecgx03.cn-hangzhou.personal.cr.aliyuncs.com/chenzh2004/alpine:3.20

# 添加安全标签
LABEL maintainer="FormalLangLab Team"
LABEL version="1.0"
LABEL description="AI Service for FormalLangLab Project"

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

# 复制迁移文件
COPY migrations/ /app/migrations/

# 复制qacache资源
COPY asset/ /app/asset/

# 暴露服务端口（使用环境变量允许动态配置）
EXPOSE 8082 6062 4042

# 运行应用（使用exec形式的CMD以确保信号能正确传递）
CMD ["/app/ai-server"]
