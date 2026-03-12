# 编译
FROM golang:1.21-alpine AS builder
WORKDIR /app
# 启用 CGO_ENABLED=0 确保生成的二进制文件在 alpine 运行不依赖动态库
RUN apk add --no-cache git
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o fluxbox main.go

# 运行
FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /root/

# 拷贝二进制
COPY --from=builder /app/fluxbox .
# 拷贝静态资源 (web 目录)
COPY --from=builder /app/web ./web

# 创建持久化目录
RUN mkdir data

EXPOSE 10504
VOLUME ["/root/data"]

CMD ["./fluxbox"]