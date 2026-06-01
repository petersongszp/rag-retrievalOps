# 多阶段构建 - 第一阶段：构建Go应用
# 【重要】这里改成和 go.mod 一致的 go 版本
FROM golang:1.25.1-alpine AS builder

# 安装必要的工具
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./

# 配置 Go 模块代理（使用国内镜像加速）
ENV GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct
ENV GOSUMDB=off

# 下载依赖
RUN go mod download

# 复制整个后端代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -installsuffix cgo -o main ./cmd/server/main.go

# 多阶段构建 - 第二阶段：运行时镜像
FROM alpine:3.18

# 安装运行时依赖
# 安装运行时依赖 (包含 OCR 工具链: poppler-utils 用于 PDF 转图片, tesseract-ocr 用于文字识别)
#RUN apk add --no-cache ca-certificates tzdata curl poppler-utils tesseract-ocr tesseract-ocr-data-chi_sim
RUN apk add --no-cache 

# 设置时区
ENV TZ=Asia/Shanghai

# 创建应用用户
RUN addgroup -g 1001 appuser && \
    adduser -D -u 1001 -G appuser appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/main .

# 复制配置文件
COPY config.yaml .

# 复制 go.mod（resolveBackendRoot 需要以此定位项目根目录）
COPY go.mod .

# 创建日志和报告目录
RUN mkdir -p logs runtime/eval-reports && chown -R appuser:appuser /app

# 切换到非root用户
USER appuser

# 暴露端口
EXPOSE 8899

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f -X OPTIONS http://localhost:8899/health || exit 1

# 启动应用
CMD ["./main"]
