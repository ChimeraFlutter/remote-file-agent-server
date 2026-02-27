# 启动指南

本文档说明如何启动 Remote File Agent Server。

## 前置要求

- Go 1.21 或更高版本
- 已配置 `config.yaml` 文件

## 配置步骤

首次运行前，需要创建配置文件：

```bash
cp config.yaml.example config.yaml
```

编辑 `config.yaml`，至少需要设置以下项：

- `server.admin_password`: Web 管理台登录密码
- `server.agent_enroll_token`: Agent 设备注册令牌

## 运行方式

### 方式一：直接运行（开发模式）

适合开发和调试，无需编译，直接运行源码：

```bash
go run ./cmd/server
```

**优点：**
- 快速启动，无需编译步骤
- 代码修改后直接运行即可看到效果

**缺点：**
- 每次启动都需要重新编译
- 启动速度较慢

### 方式二：先编译再运行（推荐）

适合生产环境和长期运行：

```bash
# 1. 编译
go build -o server.exe ./cmd/server

# 2. 运行
./server.exe
```

**优点：**
- 启动速度快
- 可以将编译后的二进制文件部署到其他机器
- 性能更好

**缺点：**
- 代码修改后需要重新编译

## 验证服务

服务启动后，可以通过以下方式验证：

### 1. 检查健康状态

```bash
curl http://localhost:18120/health
```

预期返回：
```json
{"status":"ok","version":"1.0.0"}
```

### 2. 访问 Web 管理台

打开浏览器访问：
```
http://localhost:18120/admin/login.html
```

使用配置文件中设置的 `admin_password` 登录。

## 常见问题

### 端口被占用

如果看到 `bind: address already in use` 错误，说明端口 18120 已被占用。

解决方法：
1. 修改 `config.yaml` 中的 `server.port` 为其他端口
2. 或者停止占用该端口的其他程序

### 配置文件未找到

如果看到 `config.yaml not found` 错误，请确保：
1. 已从 `config.yaml.example` 复制配置文件
2. 在项目根目录下运行程序

### 数据库初始化失败

首次运行时，程序会自动创建数据库和必要的目录。如果失败，请检查：
1. 配置文件中的 `storage.db_path` 路径是否有写入权限
2. 配置文件中的 `storage.objects_dir` 路径是否有写入权限

## 停止服务

按 `Ctrl+C` 即可优雅停止服务。
