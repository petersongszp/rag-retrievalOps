# syntax=docker/dockerfile:1

FROM node:18-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm,id=npm-cache \
    npm ci --prefer-offline

FROM node:18-alpine AS builder

# 设置工作目录
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ARG NEXT_PUBLIC_API_BASE_URL
ENV NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL:-http://localhost:8899/api}
RUN --mount=type=cache,target=/app/.next/cache,id=next-cache \
    npm run build

FROM node:18-alpine AS runner
RUN apk add --no-cache curl

# 设置时区
ENV TZ=Asia/Shanghai

# 设置环境变量
ENV NODE_ENV=production

# 创建应用用户
RUN addgroup -g 1001 appuser && \
    adduser -D -u 1001 -G appuser appuser

# 设置工作目录
WORKDIR /app

# 只复制必要文件
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package*.json ./
# COPY --from=builder /app/public ./public/ 2>/dev/null || true
RUN chown -R appuser:appuser /app
# 切换到非root用户
USER appuser

# 暴露端口
EXPOSE 3000

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:3000 || exit 1

# 启动应用
CMD ["npm", "start"]
