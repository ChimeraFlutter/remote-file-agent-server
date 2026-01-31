# Remote File Agent Server

Remote File Manager 的 Golang 后端服务器。

## 功能

- WebSocket 实时通信
- 设备注册和管理
- 文件浏览和操作
- Web 管理台
- SQLite 数据存储

## 快速开始

### 编译

```bash
# 编译为当前平台
go build -o server ./cmd/server

# 交叉编译 macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o server-darwin-amd64 ./cmd/server

# 交叉编译 macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o server-darwin-arm64 ./cmd/server

# 交叉编译 Linux
GOOS=linux GOARCH=amd64 go build -o server-linux-amd64 ./cmd/server

# 交叉编译 Windows
GOOS=windows GOARCH=amd64 go build -o server-windows-amd64.exe ./cmd/server
```

### 配置

复制配置文件并修改：

```bash
cp config.yaml.example config.yaml
```

配置项说明：

```yaml
server:
  host: "0.0.0.0"
  port: 18120
  admin_password: "your-password"        # Web 管理台密码
  agent_enroll_token: "your-token"       # Agent 注册令牌

storage:
  objects_dir: "./data/objects"          # 文件存储目录
  db_path: "./data/meta.sqlite"          # 数据库路径
  max_file_size_gb: 10                   # 最大文件大小

security:
  session_timeout_minutes: 60            # 会话超时
  download_token_timeout_minutes: 10     # 下载令牌超时

websocket:
  ping_interval_seconds: 10
  pong_timeout_seconds: 30
  heartbeat_interval_seconds: 15

logging:
  level: "info"
  output: "stdout"
```

### 运行

```bash
./server
```

服务器将在 `http://localhost:18120` 启动。

### 验证

```bash
curl http://localhost:18120/health
# 返回: {"status":"ok","version":"1.0.0"}
```

### Web 管理台

- 登录页面: `http://localhost:18120/admin/login.html`
- 默认密码: 配置文件中的 `admin_password`

## Docker 部署

```bash
# 构建镜像
docker build -t rfm-server .

# 运行容器
docker run -d -p 18120:18120 -v ./data:/app/data rfm-server

# 或使用 docker-compose
docker-compose up -d
```

## API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/ws/agent` | WebSocket | Agent 连接 |
| `/api/admin/login` | POST | 管理员登录 |
| `/api/admin/devices` | GET | 设备列表 |
| `/api/admin/devices/{id}/files` | GET | 浏览设备文件 |

## 目录结构

```
.
├── cmd/server/          # 主程序入口
├── internal/            # 内部模块
│   ├── admin/          # 管理台 API
│   ├── audit/          # 审计日志
│   ├── devices/        # 设备管理
│   ├── objects/        # 文件对象存储
│   ├── rpc/            # RPC 框架
│   └── ws/             # WebSocket 管理
├── migrations/          # 数据库迁移
├── pkg/                 # 公共包
│   └── config/         # 配置管理
├── web/admin/          # Web 管理台静态文件
├── config.yaml.example # 配置模板
├── Dockerfile          # Docker 构建文件
└── docker-compose.yml  # Docker Compose 配置
```

## License

MIT License
