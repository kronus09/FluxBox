# FluxBox - 影视Box 多源聚合引擎

FluxBox 是一个专为 影视Box 设计的多源聚合引擎，支持聚合和解密多个 影视Box 配置源，提供统一的订阅地址。

## ✨ 特性

- **多源聚合**：支持聚合多个 影视Box 配置源
- **自动脱壳**：支持 AES-CBC、AES-ECB、Base64 及图像隐写术解密
- **智能过滤**：支持关键词过滤，自动排除直播、教育等类型站点
- **聚合模式**：支持"最快站点"（取前120个）和"全部站点"两种模式
- **计划任务**：支持定时自动聚合，可设置每天或每周执行
- **实时状态**：显示同步状态和源状态
- **Web 管理**：提供简洁的 Web 界面进行管理
- **Docker 支持**：支持 Docker 部署，数据持久化
- **多架构支持**：支持 amd64 和 arm64 架构
- **完全本地化**：无外链资源依赖，可离线运行

## 🛠️ 安装方法

### 方法一：使用 Docker Compose（推荐）

1. **创建 docker-compose.yml 文件**
   ```yaml
   services:
     fluxbox:
       image: ghcr.io/kronus09/fluxbox:latest
       container_name: fluxbox
       ports:
         - "20504:20504"
       volumes:
         - ./data:/app/data
       restart: unless-stopped
       environment:
         - TZ=Asia/Shanghai
         - GIN_MODE=release
   ```

2. **启动服务**
   ```bash
   docker-compose up -d
   ```

3. **访问 Web 界面**
   打开浏览器访问：`http://localhost:20504`

### 方法二：使用 Docker 命令

```bash
docker run -d \
  --name fluxbox \
  -p 20504:20504 \
  -v $(pwd)/data:/app/data \
  --restart unless-stopped \
  -e TZ=Asia/Shanghai \
  -e GIN_MODE=release \
  ghcr.io/kronus09/fluxbox:latest
```

### 方法三：直接运行

1. **安装 Go 环境**（版本 1.21+）

2. **克隆项目**
   ```bash
   git clone https://github.com/kronus09/fluxbox.git
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

### 3. 聚合设置

点击标题栏的 ⚙️ 图标打开设置面板：

- **聚合模式**：
  - 最快站点：按响应时间排序，取前120个最快站点
  - 全部站点：聚合所有站点
- **过滤关键词**：
  - 默认过滤：直播、儿童、启蒙、教育、课堂、学习、少儿、预告
  - 可自定义添加或删除关键词
  - 点击"重置默认"恢复初始设置
- **计划任务**：
  - 启用定时聚合：开启后可设置自动执行时间
  - 执行频率：每天或每周
  - 执行时间：设置具体执行时间点
  - 执行日期：选择每周模式下执行的日期（周一至周日）

### 4. 复制订阅地址

1. 点击 "复制订阅" 按钮
2. 订阅地址会自动复制到剪贴板
3. 将订阅地址添加到 影视Box 中

### 5. 管理源

- **修改源**：点击源右侧的 "修改" 按钮，在弹出的模态框中修改源信息
- **删除源**：点击源右侧的 "删除" 按钮
- **测试连接**：点击 "测试连接" 按钮检测源是否有效
- **批量测试**：勾选多个源后点击 "测试连接" 批量检测

## ⚙️ 配置说明

### 数据持久化

- Docker 部署：数据存储在当前目录的 `data/` 文件夹中（绑定挂载）
- 直接运行：数据存储在 `data/` 目录中

### 主要文件

- `data/sources.json`：存储源配置
- `data/config_cache.json`：存储聚合后的配置缓存
- `data/filter_words.json`：存储过滤关键词配置
- `data/schedule.json`：存储计划任务配置

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
