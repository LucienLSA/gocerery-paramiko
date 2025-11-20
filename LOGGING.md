# 日志方案说明

## 快速开始

### 配置日志

编辑 `etc/gocerery-api.yaml`：

```yaml
Log:
  ServiceName: gocerery
  Mode: file          # file: 文件模式, console: 控制台模式
  Path: logs          # 日志文件路径
  Level: info         # debug, info, error
  Compress: true      # 是否压缩
  KeepDays: 7         # 保留天数
```

### 查看日志

```bash
# 实时查看 Worker 日志
tail -f logs/gocerery-worker.log

# 搜索错误日志
grep -i error logs/*.log

# 查看特定任务的日志
grep "task_id" logs/*.log
```

## 一、日志架构

本项目采用统一的日志方案，使用 **go-zero 的 logx** 作为 Go 代码的日志框架，使用 **Python logging** 作为 Python 脚本的日志框架。

### 日志层次

```
┌─────────────────────────────────┐
│  HTTP API Server (go-zero logx) │
│  - 日志文件: logs/gocerery-api.log │
└─────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────┐
│  Celery Worker (go-zero logx)   │
│  - 日志文件: logs/gocerery-worker.log │
└─────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────┐
│  Python Scripts (logging)       │
│  - 日志文件: logs/ssh_executor.log │
│  - 日志文件: logs/ssh_uploader.log │
│  - stderr: 被 Go Worker 捕获      │
└─────────────────────────────────┘
```

## 二、日志配置

### 2.1 配置文件 (`etc/gocerery-api.yaml`)

```yaml
# 日志配置
Log:
  ServiceName: gocerery          # 服务名称（用于日志文件名）
  Mode: file                     # 日志模式：file（文件）、console（控制台）、volume（卷）
  Path: logs                      # 日志文件路径（Mode=file 时生效）
  Level: info                     # 日志级别：debug, info, error
  Compress: true                  # 是否压缩日志文件
  KeepDays: 7                     # 日志保留天数
  StackCooldownMillis: 100        # 堆栈冷却时间（毫秒）
```

### 2.2 日志级别说明

- **debug**：详细调试信息，包括所有操作细节
- **info**：一般信息，包括任务执行、连接状态等
- **error**：错误信息，包括异常、失败等

### 2.3 日志模式说明

- **file**：日志输出到文件（推荐生产环境）
  - 日志文件位置：`logs/{ServiceName}.log`
  - 支持日志轮转和压缩
- **console**：日志输出到控制台（推荐开发环境）
- **volume**：日志输出到卷（容器环境）

## 三、日志文件结构

### 3.1 日志文件位置

```
logs/
├── gocerery-api.log          # HTTP API Server 日志
├── gocerery-worker.log       # Celery Worker 日志
├── ssh_executor.log          # 命令执行脚本日志
└── ssh_uploader.log          # 文件上传脚本日志
```

### 3.2 日志文件命名规则

- Go 服务日志：`{ServiceName}-{type}.log`
  - `gocerery-api.log`：API Server
  - `gocerery-worker.log`：Worker
- Python 脚本日志：`{script_name}.log`
  - `ssh_executor.log`：命令执行脚本
  - `ssh_uploader.log`：文件上传脚本

## 四、日志内容

### 4.1 Go Worker 日志

#### 日志格式

```
2025-11-20 10:30:15 [INFO] [WORKER] initializing Celery worker...
2025-11-20 10:30:15 [INFO] [WORKER] connecting to broker broker=redis://127.0.0.1:6379/0
2025-11-20 10:30:15 [INFO] [WORKER] registering task task=tasks.execute_ssh
2025-11-20 10:30:20 [INFO] [WORKER] received task, parsing payload...
2025-11-20 10:30:20 [INFO] [WORKER] task parsed proxy=192.168.110.130:22 targets=1 commands=2
2025-11-20 10:30:20 [INFO] [WORKER] executing script script=scripts/ssh_executor.py
2025-11-20 10:30:21 [INFO] [WORKER] script execution completed stdout_length=301
2025-11-20 10:30:21 [INFO] [WORKER] task completed success_count=1 total_count=1
```

#### 关键日志点

- **任务接收**：`[WORKER] received task, parsing payload...`
- **任务解析**：`[WORKER] task parsed proxy=... targets=... commands=...`
- **脚本执行**：`[WORKER] executing script script=...`
- **执行完成**：`[WORKER] script execution completed stdout_length=...`
- **任务结果**：`[WORKER] task completed success_count=... total_count=...`
- **错误信息**：`[WORKER] failed to ... error=...`

### 4.2 Python 脚本日志

#### 日志格式

```
2025-11-20 10:30:20 [INFO] __main__: Connecting to bastion 192.168.110.130:22
2025-11-20 10:30:20 [INFO] __main__: Opening channel to target 192.168.110.131:22
2025-11-20 10:30:20 [INFO] __main__: Successfully connected to target 192.168.110.131
2025-11-20 10:30:20 [INFO] __main__: Starting command execution on thin1 (192.168.110.131)
2025-11-20 10:30:20 [INFO] __main__: Executing command 1/2 on thin1: pwd && uname -a
2025-11-20 10:30:20 [DEBUG] __main__: Command 1 on thin1 completed with exit_code=0
2025-11-20 10:30:20 [INFO] __main__: Command execution on thin1 completed, success=True
```

#### 关键日志点

- **连接跳板机**：`Connecting to bastion ...`
- **连接目标**：`Successfully connected to target ...`
- **执行命令**：`Executing command ... on ...`
- **命令完成**：`Command ... completed with exit_code=...`
- **任务完成**：`Command execution on ... completed, success=...`
- **错误信息**：`Error executing commands on ...: ...`

### 4.3 API Server 日志

#### 日志格式

```
2025-11-20 10:30:15 [INFO] submitted ssh task task_id=xxx-xxx-xxx targets=1
2025-11-20 10:30:25 [INFO] task xxx-xxx-xxx not finished yet: ...
```

## 五、查看日志

### 5.1 实时查看日志

```bash
# 查看 Worker 日志
tail -f logs/gocerery-worker.log

# 查看 API Server 日志
tail -f logs/gocerery-api.log

# 查看 Python 脚本日志
tail -f logs/ssh_executor.log
tail -f logs/ssh_uploader.log

# 同时查看多个日志文件
tail -f logs/*.log
```

### 5.2 搜索日志

```bash
# 搜索错误日志
grep -i "error\|failed\|exception" logs/*.log

# 搜索特定任务
grep "task_id" logs/*.log

# 搜索特定服务器
grep "192.168.110.131" logs/*.log

# 搜索特定命令
grep "pwd" logs/ssh_executor.log
```

### 5.3 统计日志

```bash
# 统计成功/失败的任务数
grep "task completed" logs/gocerery-worker.log | \
  grep -o "[0-9]*/[0-9]* targets succeeded"

# 统计错误数量
grep -c "error" logs/*.log

# 查看最近的错误
grep "error" logs/*.log | tail -20
```

## 六、日志级别调整

### 6.1 修改配置文件

编辑 `etc/gocerery-api.yaml`：

```yaml
Log:
  Level: debug  # 改为 debug 获取更详细的日志
```

### 6.2 Python 脚本日志级别

Python 脚本的日志级别通过命令行参数传递，Worker 会自动根据配置传递：

- 如果配置中 `Log.Level = "debug"`，Python 脚本会使用 `DEBUG` 级别
- 如果配置中 `Log.Level = "info"`，Python 脚本会使用 `INFO` 级别

### 6.3 启用 Paramiko 调试日志

如果需要查看 Paramiko 的详细连接日志，可以修改 Python 脚本：

在 `scripts/ssh_executor.py` 或 `scripts/ssh_uploader.py` 中：

```python
# 设置 Paramiko 日志级别为 DEBUG
paramiko_logger = logging.getLogger("paramiko")
paramiko_logger.setLevel(logging.DEBUG)  # 改为 DEBUG
```

## 七、日志轮转

### 7.1 自动轮转

go-zero 的 logx 支持自动日志轮转：

- **按大小轮转**：当日志文件达到一定大小时自动轮转
- **按时间轮转**：每天自动创建新的日志文件
- **自动压缩**：旧日志文件自动压缩（如果 `Compress: true`）
- **自动清理**：超过 `KeepDays` 天的日志自动删除

### 7.2 手动清理

```bash
# 删除 7 天前的日志
find logs/ -name "*.log" -mtime +7 -delete

# 删除压缩的日志文件
find logs/ -name "*.log.gz" -mtime +30 -delete
```

## 八、生产环境建议

### 8.1 日志配置

```yaml
Log:
  ServiceName: gocerery
  Mode: file
  Path: /var/log/gocerery    # 使用绝对路径
  Level: info                 # 生产环境使用 info，避免 debug 日志过多
  Compress: true
  KeepDays: 30                # 保留 30 天
  StackCooldownMillis: 100
```

### 8.2 日志监控

建议使用日志收集工具（如 ELK、Loki、Fluentd）收集和分析日志：

```bash
# 示例：使用 filebeat 收集日志
filebeat.inputs:
- type: log
  paths:
    - /var/log/gocerery/*.log
  fields:
    service: gocerery
```

### 8.3 日志告警

可以配置日志告警规则：

- **错误率告警**：当错误日志数量超过阈值时告警
- **任务失败告警**：当任务失败率超过阈值时告警
- **连接失败告警**：当 SSH 连接失败次数过多时告警

## 九、调试技巧

### 9.1 启用详细日志

1. 修改配置文件 `Log.Level = "debug"`
2. 重启 Worker 和 API Server
3. 查看日志文件获取详细信息

### 9.2 查看特定任务的日志

```bash
# 1. 提交任务，获取 task_id
task_id="xxx-xxx-xxx"

# 2. 在日志中搜索该 task_id
grep "$task_id" logs/*.log

# 3. 查看该任务的所有相关日志
grep -A 10 -B 10 "$task_id" logs/*.log
```

### 9.3 查看 Python 脚本的 stderr

Python 脚本的 stderr 会被 Go Worker 捕获并记录：

```bash
# 在 Worker 日志中搜索 stderr
grep "stderr" logs/gocerery-worker.log
```

### 9.4 直接运行 Python 脚本测试

```bash
# 直接运行脚本，查看日志输出
python3 scripts/ssh_executor.py \
  --bastion '{"host":"...","port":22,"user":"...","password":"..."}' \
  --targets '[{"name":"test","host":"...","port":22,"user":"...","password":"..."}]' \
  --commands '["pwd","uname -a"]' \
  --concurrency 1 \
  --timeout 30 \
  --log-level DEBUG \
  --log-file /tmp/test.log
```

## 十、常见问题

### 10.1 日志文件未生成

**问题**：日志文件没有生成

**解决方案**：
1. 检查日志目录权限：`chmod 755 logs`
2. 检查配置文件中的 `Log.Path` 是否正确
3. 检查 `Log.Mode` 是否为 `file`

### 10.2 日志文件过大

**问题**：日志文件占用磁盘空间过大

**解决方案**：
1. 调整日志级别为 `info` 或 `error`
2. 减少 `KeepDays` 的值
3. 启用日志压缩：`Compress: true`

### 10.3 日志信息不够详细

**问题**：需要更详细的调试信息

**解决方案**：
1. 设置 `Log.Level = "debug"`
2. 在 Python 脚本中启用 Paramiko DEBUG 日志
3. 查看 Worker 日志中的详细输出

### 10.4 Python 脚本日志未输出

**问题**：Python 脚本的日志没有输出到文件

**解决方案**：
1. 检查日志文件路径是否正确
2. 检查日志目录是否存在且有写权限
3. 查看 Worker 日志中的 stderr 输出

## 十一、日志示例

### 11.1 正常执行日志

```
# Worker 日志
2025-11-20 10:30:15 [INFO] [WORKER] initializing Celery worker...
2025-11-20 10:30:15 [INFO] [WORKER] connecting to broker broker=redis://127.0.0.1:6379/0
2025-11-20 10:30:20 [INFO] [WORKER] received task, parsing payload...
2025-11-20 10:30:20 [INFO] [WORKER] task parsed proxy=192.168.110.130:22 targets=1 commands=2
2025-11-20 10:30:20 [INFO] [WORKER] executing script script=scripts/ssh_executor.py
2025-11-20 10:30:21 [INFO] [WORKER] script execution completed stdout_length=301
2025-11-20 10:30:21 [INFO] [WORKER] task completed success_count=1 total_count=1

# Python 脚本日志
2025-11-20 10:30:20 [INFO] Connecting to bastion 192.168.110.130:22
2025-11-20 10:30:20 [INFO] Successfully connected to target 192.168.110.131
2025-11-20 10:30:20 [INFO] Starting command execution on thin1 (192.168.110.131)
2025-11-20 10:30:20 [INFO] Executing command 1/2 on thin1: pwd && uname -a
2025-11-20 10:30:20 [INFO] Command execution on thin1 completed, success=True
```

### 11.2 错误执行日志

```
# Worker 日志
2025-11-20 10:30:20 [ERROR] [WORKER] script execution failed error=exit status 1 stderr=...
2025-11-20 10:30:21 [INFO] [WORKER] task completed success_count=0 total_count=1

# Python 脚本日志
2025-11-20 10:30:20 [ERROR] Error executing commands on thin1: AuthenticationException: Authentication failed
2025-11-20 10:30:20 [ERROR] Traceback (most recent call last):
  ...
```

## 十二、总结

本日志方案提供了统一的日志格式、灵活的日志级别、自动日志轮转、详细的调试信息和易于查看的日志结构，帮助快速定位问题、监控系统运行状态。

