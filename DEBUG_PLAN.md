# 🔍 服务器卡死问题调试方案

## 问题描述
- 服务器在下载文件10分钟后卡死
- 登录接口变慢或无响应
- WAL文件增长到4MB

## 调试工具已部署

### 1. 请求追踪中间件 (`internal/middleware/debug.go`)
- 记录每个请求的开始/结束时间
- 监控数据库连接池状态（打开/使用中/空闲）
- 监控goroutine数量变化
- 自动检测慢请求

### 2. 数据库查询日志 (`internal/dblog/logger.go`)
- 记录每个SQL查询及执行时间
- 自动标记超过100ms的慢查询
- 帮助定位数据库瓶颈

### 3. 端点测试脚本 (`test_endpoints.sh`)
- 逐个测试各个端点
- 并发测试
- 测量响应时间

## 已知问题点

### 1. 数据库配置
**之前**: `SetMaxOpenConns(1)` - 只有1个连接，审计日志会阻塞其他操作
**现在**: `SetMaxOpenConns(10)` - 允许并发读写

### 2. WAL文件增长
- 当前WAL文件: 4.0MB
- 原因: 审计日志频繁写入，checkpoint不及时
- 影响: 读取性能下降

### 3. 10分钟超时
- 配置中的download_token_timeout: 10分钟
- 需要检查是否有相关的清理任务在10分钟时触发

## 下一步调试步骤

### 步骤1: 集成调试中间件
修改 `cmd/server/main.go`，添加调试中间件到路由

### 步骤2: 启动服务器并监控
```bash
# 启动服务器
cd C:\Users\Administrator\remote-file-agent-server
bin\server.exe

# 在另一个终端监控日志
tail -f server.log
```

### 步骤3: 重现问题
1. 开始下载文件
2. 等待10分钟
3. 观察日志输出
4. 检查哪个请求卡住了

### 步骤4: 分析日志
查找以下信息:
- 哪个端点在10分钟时被调用
- 数据库连接池是否耗尽 (db_in_use == db_open_conns)
- goroutine数量是否异常增长
- 是否有慢查询

### 步骤5: 数据库健康检查
```bash
# 检查WAL文件大小
ls -lh data/meta.sqlite*

# 手动触发checkpoint
sqlite3 data/meta.sqlite "PRAGMA wal_checkpoint(TRUNCATE);"

# 查看审计日志数量
sqlite3 data/meta.sqlite "SELECT COUNT(*) FROM audit_logs;"
```

## 可能的根本原因

### 假设1: 审计日志阻塞
- **症状**: 频繁的审计日志写入占用数据库连接
- **验证**: 查看日志中的SQL EXEC时间
- **解决**: 已实现channel-based worker + 增加连接池

### 假设2: Goroutine泄漏
- **症状**: goroutine数量持续增长
- **验证**: 查看日志中的goroutine_delta
- **解决**: 需要找到泄漏点并修复

### 假设3: WebSocket连接问题
- **症状**: 10分钟后WebSocket超时或清理触发阻塞
- **验证**: 检查WebSocket相关的ticker和cleanup
- **解决**: 优化清理逻辑，避免阻塞

### 假设4: RPC超时处理
- **症状**: RPC请求超时后没有正确清理
- **验证**: 查看pending requests是否堆积
- **解决**: 确保超时后正确清理资源

### 假设5: 文件下载长连接
- **症状**: 大文件下载占用连接，10分钟后token过期导致问题
- **验证**: 检查下载过程中的连接状态
- **解决**: 优化下载超时处理

## 监控指标

关键指标:
- **db_in_use**: 正在使用的数据库连接数 (应该 < 10)
- **goroutines**: goroutine总数 (不应持续增长)
- **request_duration**: 请求耗时 (应该 < 1秒)
- **sql_duration**: SQL查询耗时 (应该 < 100ms)

异常阈值:
- db_in_use == 10: 连接池耗尽
- goroutines > 1000: 可能有泄漏
- request_duration > 5s: 请求阻塞
- sql_duration > 1s: 数据库问题

## 临时缓解措施

如果问题再次发生:
1. 重启服务器: `taskkill //F //IM server.exe && bin\server.exe`
2. 清理WAL文件: `sqlite3 data/meta.sqlite "PRAGMA wal_checkpoint(TRUNCATE);"`
3. 清理审计日志: `sqlite3 data/meta.sqlite "DELETE FROM audit_logs WHERE timestamp < strftime('%s', 'now', '-7 days');"`

## 长期解决方案

1. **定期WAL checkpoint**: 添加定时任务每5分钟执行一次
2. **审计日志轮转**: 自动删除7天前的日志
3. **连接池监控**: 添加Prometheus metrics
4. **请求超时**: 为所有HTTP请求添加超时
5. **Graceful shutdown**: 确保服务器关闭时清理所有资源
