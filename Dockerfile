# 多阶段构建：编译 Go → 构建前端 → 运行镜像
# Stage 1: 编译 Go 后端
FROM golang:1.22-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/api/

# Stage 2: 构建前端
FROM node:20-alpine AS web-builder
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci --ignore-scripts 2>/dev/null || npm install
COPY web/ .
RUN npm run build

# Stage 3: 运行镜像
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata curl
WORKDIR /app

# 复制 Go 二进制
COPY --from=go-builder /app/server .

# 复制前端构建产物
COPY --from=web-builder /app/web/dist ./web/dist

# 复制配置文件模板
COPY configs/config.yaml.example ./configs/config.yaml.example
COPY .env.example ./

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["./server"]
