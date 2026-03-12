# 第一阶段：编译阶段
FROM golang:1.21-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git

WORKDIR /app

# --- 关键优化 1：利用 Docker 层缓存机制 ---
# 先拷贝依赖文件，单独进行下载。只要 go.mod 不变，这层就不会重新执行
COPY go.mod go.sum ./
# 设置代理以解决构建失败问题
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download

# --- 关键优化 2：拷贝全量源码并编译 ---
COPY . .
# 再次 tidy 确保依赖一致性
RUN go mod tidy
# 编译：禁用 CGO 以实现全静态链接，-ldflags="-s -w" 压缩体积
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o fluxbox main.go

# 第二阶段：运行阶段
FROM alpine:latest

# 安装必要的基础库和时区数据
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /root/

# --- 关键优化 3：环境变量设置 ---
ENV TZ=Asia/Shanghai
ENV GIN_MODE=release

# 从编译阶段拷贝二进制文件
COPY --from=builder /app/fluxbox .
# 拷贝静态资源目录 (确保路径与程序内部调用一致)
COPY --from=builder /app/web ./web

# 预创建持久化目录
RUN mkdir -p data

# 声明端口
EXPOSE 10504

# 声明匿名卷
VOLUME ["/root/data"]

# 启动程序
ENTRYPOINT ["./fluxbox"]