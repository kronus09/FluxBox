# ===========================================
# 第一阶段：构建 Go 程序
# ===========================================
FROM golang:1.23-alpine AS builder

# 设置 Go 环境
ENV CGO_ENABLED=0 GOOS=linux
WORKDIR /app

# 优化点：使用通配符拷贝，防止缺少 go.sum 导致构建中断
COPY go.mod go.sum* ./
# 只有当 go.mod 包含依赖时才运行下载，并使用官方全球代理（Actions 极速）
RUN go env -w GOPROXY=https://proxy.golang.org,direct
RUN if [ -f go.sum ]; then go mod download; fi

COPY . .
# 编译并压缩体积
RUN go build -ldflags="-s -w" -o fluxbox .

# ===========================================
# 第二阶段：运行镜像
# ===========================================
FROM alpine:latest

# 安装基础证书和时区数据（FluxBox 不需要 FFmpeg，保持镜像精简）
RUN apk add --no-cache ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai
# 设置为生产模式
ENV GIN_MODE=release

WORKDIR /app

# 拷贝二进制文件
COPY --from=builder /app/fluxbox .
# 拷贝 Web 静态资源
COPY --from=builder /app/web ./web

# --- FluxBox 特有的持久化目录 ---
# 确保数据目录存在并有读写权限
RUN mkdir -p ./data && chmod -R 777 ./data

# 暴露端口 (FluxBox 默认端口)
EXPOSE 10504

# 启动命令
CMD ["./fluxbox"]