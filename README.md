# FluxBox - 影视Box 多源聚合引擎

FluxBox 是一个专为 影视Box 设计的多源聚合引擎，支持聚合和解密多个 影视Box 配置源，提供统一的订阅地址。

## ✨ 特性

- **多源聚合**：支持聚合多个 影视Box 配置源
- **自动脱壳**：支持 AES-CBC、AES-ECB、Base64 及图像隐写术解密
- **实时状态**：显示同步状态和源状态
- **Web 管理**：提供简洁的 Web 界面进行管理
- **Docker 支持**：支持 Docker 部署，数据持久化
- **多架构支持**：支持 amd64 和 arm64 架构

## 🛠️ 安装方法

### 方法一：使用 Docker Compose（推荐）

1. **修改 docker-compose.yml**
   ```bash
   docker-compose up -d
   ```

2. **启动服务**
   ```
   services:
  fluxbox:
    image: ghcr.io/kronus09/fluxbox:latest
    container_name: fluxbox
    ports:
      - "10504:10504"
    volumes:
      - fluxbox_data:/root/data
    restart: unless-stopped
    environment:
      - TZ=Asia/Shanghai

volumes:
  fluxbox_data:
   ```

3. **访问 Web 界面**
   打开浏览器访问：`http://localhost:10504`

### 方法二：使用 Docker 命令

```bash
docker run -d \
  --name fluxbox \
  -p 10504:10504 \
  -v fluxbox_data:/root/data \
  --restart unless-stopped \
  ghcr.io/yourusername/fluxbox:latest
```

### 方法三：直接运行

1. **安装 Go 环境**（版本 1.21+）

2. **克隆项目**
   ```bash
   git clone https://github.com/yourusername/fluxbox.git
   cd fluxbox
   ```

3. **构建并运行**
   ```bash
   go mod tidy
   go build -o fluxbox main.go
   ./fluxbox
   ```

## 🚀 使用方法

### 1. 添加源

1. 在 Web 界面的 "添加新源" 部分填写源名称和 URL
2. 点击 "确认添加" 按钮

### 2. 同步聚合

1. 点击 "同步聚合" 按钮开始聚合过程
2. 状态显示栏会显示同步进度
3. 同步完成后，会显示聚合的站点数量

### 3. 复制订阅地址

1. 点击 "复制订阅" 按钮
2. 订阅地址会自动复制到剪贴板
3. 将订阅地址添加到 影视Box 中

### 4. 管理源

- **修改源**：点击源右侧的 "修改" 按钮，在弹出的模态框中修改源信息
- **删除源**：点击源右侧的 "删除" 按钮

## ⚙️ 配置说明

### 数据持久化

- Docker 部署：数据存储在 `fluxbox_data` 卷中
- 直接运行：数据存储在 `data/` 目录中

### 主要文件

- `data/sources.json`：存储源配置
- `data/config_cache.json`：存储聚合后的配置缓存

### 环境变量

- `TZ`：时区设置，默认 `Asia/Shanghai`

## 📝 开发

### 项目结构

```
├── api/           # API 处理
├── models/        # 数据模型
├── parser/        # 配置解析
├── web/           # Web 界面
│   ├── index.html # 主页面
│   └── static/    # 静态资源
├── main.go        # 主入口
├── Dockerfile     # Docker 构建文件
└── docker-compose.yml # Docker Compose 配置
```

### 构建 Docker 镜像

```bash
docker build -t fluxbox .
```

## 🔧 常见问题

### 1. 同步失败

- 检查源 URL 是否可访问
- 检查网络连接
- 查看容器日志：`docker logs fluxbox`

### 2. 订阅地址无法访问

- 检查容器是否正常运行：`docker ps`
- 检查端口映射是否正确
- 检查防火墙设置

### 3. 数据丢失

- 确保使用了数据卷持久化
- 定期备份 `data/` 目录

## 📄 许可证

MIT License
