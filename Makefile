# Makefile for ai service
# Usage:
#   make lint          # 运行代码质量检查
#   make lint-fix      # 运行代码质量检查并自动修复
#   make tag           # 为服务打标签并推送
#   make build         # 构建并推送镜像
#   make deploy        # 部署服务

.PHONY: lint lint-fix tag build deploy

# ========== 代码质量检查 ==========
lint:
	@echo "🔍 运行 ai 服务代码质量检查..."
	@golangci-lint run

lint-fix:
	@echo "🔧 运行 ai 服务代码质量检查并自动修复..."
	@golangci-lint run --fix || true

# ========== 为服务打标签 ==========
# 用法: make tag VERSION=v1.0.1
tag:
	@if [ -z "$(VERSION)" ]; then \
		echo "错误：缺少 VERSION 参数"; \
		echo "用法: make tag VERSION=v1.0.1"; \
		exit 1; \
	fi
	@echo "📦 为 ai 服务打标签 $(VERSION)..."
	@git tag $(VERSION)
	@git push origin tag $(VERSION)
	@echo "✅ ai 服务标签完成"

# ========== 构建并推送镜像 ==========
build:
	@echo "📦 构建 ai 服务镜像..."
	@docker build -t crpi-tcnuencv1iecgx03.cn-hangzhou.personal.cr.aliyuncs.com/chenzh2004/formallanglab-ai .
	@echo "📤 推送 ai 服务镜像..."
	@docker push crpi-tcnuencv1iecgx03.cn-hangzhou.personal.cr.aliyuncs.com/chenzh2004/formallanglab-ai
	@echo "✅ ai 服务镜像构建并推送完成"

# ========== 部署服务 ==========
deploy:
	@echo "🚀 开始部署 ai 服务..."
	@echo "📦 步骤 1: 构建 ai 服务镜像..."
	@docker build -t crpi-tcnuencv1iecgx03.cn-hangzhou.personal.cr.aliyuncs.com/chenzh2004/formallanglab-ai .
	@echo "📤 步骤 2: 推送 ai 服务镜像..."
	@docker push crpi-tcnuencv1iecgx03.cn-hangzhou.personal.cr.aliyuncs.com/chenzh2004/formallanglab-ai
	@echo "️  步骤 3: 停止并移除 ai 服务容器..."
	@docker compose down ai
	@echo "📥 步骤 4: 拉取最新镜像..."
	@docker compose pull ai
	@echo "🚀 步骤 5: 启动 ai 服务..."
	@docker compose up -d ai
	@echo "✅ ai 服务部署完成"
