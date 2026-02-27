# 日志配置说明

## Golang 服务器日志

已配置为按日期和时间输出到指定目录。

### 配置文件位置
`config.yaml`

### 配置项
```yaml
logging:
  level: "info"
  output: "stdout"
  log_dir: "C:/logs/remote-file-logs"
  use_rotate: true
```

### 日志文件路径格式
- 目录：`C:/logs/remote-file-logs/2026-2-27/`
- 文件名：`Golang-15-00.log`（按小时命名）

### 启动服务器
```bash
cd C:\Users\Administrator\remote-file-agent-server
go run cmd/server/main.go
```

日志将自动输出到：`C:/logs/remote-file-logs/2026-2-27/Golang-15-00.log`

---

## Python MCP 客户端日志

如果你使用 Python 作为 MCP 客户端，需要在 Python 代码中配置日志。

### 方法 1：使用 Python logging 模块

在你的 Python 脚本中添加：

```python
import logging
import os
from datetime import datetime

# 创建日志目录
log_dir = r"C:\logs\remote-file-logs"
date_dir = datetime.now().strftime("%Y-%-m-%-d")
full_log_dir = os.path.join(log_dir, date_dir)
os.makedirs(full_log_dir, exist_ok=True)

# 生成日志文件名
hour = datetime.now().strftime("%H-00")
log_file = os.path.join(full_log_dir, f"Python-{hour}.log")

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler(log_file, encoding='utf-8'),
        logging.StreamHandler()  # 同时输出到控制台
    ]
)

logger = logging.getLogger(__name__)
logger.info("Python MCP client started")
```

### 方法 2：查找 .mcp.json 配置

如果你的 Python 客户端使用 `.mcp.json` 配置文件，请检查：

1. 文件位置：`C:\Users\Administrator\Documents\canxingjian2025\.mcp.json`
2. 查看是否有 `log_file` 或类似的配置项
3. 如果没有，需要在 Python 代码中手动配置（使用方法 1）

### 示例 .mcp.json 配置
```json
{
  "server_url": "http://localhost:18120/mcp",
  "auth_token": "your-token-here",
  "log_file": "C:/logs/remote-file-logs/2026-2-27/Python-15-00.log",
  "log_level": "INFO"
}
```

---

## 查看日志

### 实时查看 Golang 日志
```bash
tail -f C:/logs/remote-file-logs/2026-2-27/Golang-15-00.log
```

### 实时查看 Python 日志
```bash
tail -f C:/logs/remote-file-logs/2026-2-27/Python-15-00.log
```

### 查看所有今天的日志
```bash
ls -lh C:/logs/remote-file-logs/2026-2-27/
```

---

## 注意事项

1. **日志轮转**：每小时会创建一个新的日志文件（例如：15-00.log, 16-00.log）
2. **目录结构**：日志按日期组织，每天一个目录
3. **权限**：确保程序有权限创建目录和写入文件
4. **磁盘空间**：定期清理旧日志，避免占用过多磁盘空间

## 清理旧日志

可以创建一个定时任务来清理 7 天前的日志：

```bash
# Windows 任务计划程序
# 或使用 PowerShell 脚本
Get-ChildItem "C:\logs\remote-file-logs" -Directory |
    Where-Object { $_.CreationTime -lt (Get-Date).AddDays(-7) } |
    Remove-Item -Recurse -Force
```
