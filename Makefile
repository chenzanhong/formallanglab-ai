# Makefile for ai service
# Usage:
#   make lint          # 运行代码质量检查
#   make lint-fix      # 运行代码质量检查并自动修复
#   make tag           # 为服务打标签并推送
#   make build         # 构建并推送镜像
#   make deploy        # 部署服务

# 镜像仓库配置
REGISTRY := crpi-tcnuencv1iecgx03.cn-hangzhou.personal.cr.aliyuncs.com/chenzh2004
IMAGE_NAME := $(REGISTRY)/formallanglab-ai

.PHONY: lint lint-fix tag build deploy

# ========== 代码质量检查 ==========
lint:
	@echo "🔍 运行 ai 服务代码质量检查..."
	@golangci-lint run

lint-fix:
	@echo "🔧 运行 ai 服务代码质量检查并自动修复..."
	@golangci-lint run --fix || true

# ========== 构建并推送镜像 ==========
# 用法：make build [T=false]
build:
	@DO_PUSH="true"; \
	if [ "$(T)" = "false" ]; then DO_PUSH="false"; fi; \
	if [ "$$DO_PUSH" = "true" ]; then echo "🚀 模式：构建并推送"; else echo "🛠️  模式：仅构建 (跳过推送)"; fi; \
	echo "📦 构建 ai 服务镜像..."; \
	docker build -t $(IMAGE_NAME) .; \
	if [ "$$DO_PUSH" = "true" ]; then \
		echo "📤 推送 ai 服务镜像..."; \
		docker push $(IMAGE_NAME); \
	else \
		echo "⏭️  跳过推送"; \
	fi; \
	echo "✅ ai 服务镜像构建完成"

# ========== 部署服务 ==========
deploy:
	@echo "🚀 开始部署 ai 服务..."
	@echo "📦 步骤 1: 构建 ai 服务镜像..."
	@docker build -t $(IMAGE_NAME) .
	@echo "📤 步骤 2: 推送 ai 服务镜像..."
	@docker push $(IMAGE_NAME)
	@echo "️  步骤 3: 停止并移除 ai 服务容器..."
	@docker compose down ai
	@echo "📥 步骤 4: 拉取最新镜像..."
	@docker compose pull ai
	@echo "🚀 步骤 5: 启动 ai 服务..."
	@docker compose up -d ai
	@echo "✅ ai 服务部署完成"
