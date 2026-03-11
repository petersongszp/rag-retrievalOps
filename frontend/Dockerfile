# 多阶段构建 - 第一阶段：构建Next.js应用
FROM node:18-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制 package.json 和 package-lock.json
COPY package*.json ./

# 安装依赖
RUN npm ci

# 复制应用代码
COPY . .

# 设置构建时的环境变量
ARG NEXT_PUBLIC_API_BASE_URL
ENV NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL:-http://localhost:8888/api}

# 构建应用
RUN npm run build

# 多阶段构建 - 第二阶段：运行时镜像
FROM node:18-alpine

# 安装运行时依赖
RUN apk add --no-cache curl

# 设置时区
ENV TZ=Asia/Shanghai

# 创建应用用户
RUN addgroup -g 1001 appuser && \
    adduser -D -u 1001 -G appuser appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制依赖和构建产物
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/package*.json ./

# 创建 public 目录（如果不存在）
RUN mkdir -p ./public

# 更改所有权
RUN chown -R appuser:appuser /app

# 切换到非root用户
USER appuser

# 暴露端口
EXPOSE 3000

# 环境变量
ENV NODE_ENV=production

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:3000 || exit 1

# 启动应用
CMD ["npm", "start"]
