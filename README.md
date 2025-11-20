## gocerery-paramiko

使用 go-zero + Paramiko 搭建的简易 SSH 任务分发服务。服务端提供 REST API，支持：
1. **命令执行**：接收需要执行的命令列表，随后通过 Paramiko 先登录跳板机，再使用用户名/密码方式登录多台目标服务器并执行指令。
2. **文件上传**：从本地目录上传文件或目录到多台远程服务器，支持递归上传整个目录结构。

### 架构一览

```
客户端 -> HTTP API(go-zero) -> gocelery(Producer)
         \                                   |
          ---------------- Celery Broker/Backend (Redis) ------------
                                                   |
                                      Celery Worker(go) -> scripts/ssh_executor.py -> Paramiko -> Bastion -> Targets[]
```

- **HTTP API**：解析请求、写入任务队列。支持命令执行和文件上传两种任务类型。
- **gocelery**：负责把任务推送到 Redis（Broker），并从 Redis（Backend）读取执行结果。
- **Worker**：单独进程，注册 `tasks.execute_ssh` 和 `tasks.upload_file`，消费队列后调用 Python 脚本执行真实 SSH 逻辑。
- **Paramiko 脚本**：
  - `ssh_executor.py`：先登录跳板机，再打开通道逐台目标主机执行命令，收集 stdout/stderr/exit_code 作为日志。
  - `ssh_uploader.py`：先登录跳板机，再打开通道逐台目标主机上传文件，支持单文件和目录递归上传。
- **查询接口**：从 Redis backend 取 `results[]`，将每台机器的执行情况返回给调用方。

### 目录结构

```
gocerery-paramiko/
├── gocerery.go                    # HTTP 服务入口
├── gocerery.api                   # API 定义文件（goctl 使用）
├── cmd/
│   └── worker/
│       └── main.go                # Celery Worker 入口
├── etc/
│   └── gocerery-api.yaml          # 服务配置文件
├── scripts/
│   ├── ssh_executor.py            # 命令执行脚本（Paramiko）
│   └── ssh_uploader.py             # 文件上传脚本（Paramiko）
├── internal/
│   ├── config/
│   │   └── config.go              # 配置结构体定义
│   ├── handler/                    # HTTP 处理器层
│   │   ├── routes.go              # 路由注册（自动生成）
│   │   ├── executesshtaskhandler.go
│   │   ├── querysshtaskhandler.go
│   │   ├── executeuploadtaskhandler.go
│   │   └── queryuploadtaskhandler.go
│   ├── logic/                      # 业务逻辑层
│   │   ├── executesshtasklogic.go
│   │   ├── querysshtasklogic.go
│   │   ├── executeuploadtasklogic.go
│   │   └── queryuploadtasklogic.go
│   ├── svc/
│   │   └── servicecontext.go      # 服务上下文（Celery 客户端）
│   ├── types/
│   │   └── types.go               # 请求/响应类型（自动生成）
│   └── worker/
│       └── ssh_worker.go           # Celery Worker 实现
├── README.md                       # 项目说明文档
├── API_IMPLEMENTATION.md           # API 实现详细说明
└── openapi.json                    # OpenAPI 文档（可选）
```

### 运行依赖

- **Go 1.20+**：用于编译和运行 HTTP API 和 Worker
- **Python 3.8+**：用于执行 Paramiko 脚本
- **Redis**：作为 Celery Broker（任务队列）和 Backend（结果存储）
- **goctl**：go-zero 代码生成工具（可选，用于重新生成代码）
- **paramiko**：Python SSH 库

#### 安装 Python 依赖

```bash
cd /home/lucien/goproject/AIDOC/gocerery-paramiko
python3 -m venv .venv
source .venv/bin/activate
pip install paramiko
```

#### 安装 Go 依赖

```bash
cd /home/lucien/goproject/AIDOC/gocerery-paramiko
go mod tidy
```

#### 安装 goctl（可选）

```bash
go install github.com/zeromicro/go-zero/tools/goctl@latest
```

#### 启动 Redis

```bash
# 使用 Docker 启动 Redis
docker run -d --name redis -p 6379:6379 redis:latest

# 或使用系统包管理器
# Ubuntu/Debian
sudo apt-get install redis-server
sudo systemctl start redis

# CentOS/RHEL
sudo yum install redis
sudo systemctl start redis
```

### 配置说明（`etc/gocerery-api.yaml`）

```yaml
Name: gocerery-api
Host: 0.0.0.0
Port: 8888

Bastion:
  Host: bastion.example.com
  Port: 22
  User: bastion
  Password: bastion-password

Targets:
  - Name: web-server
    Host: 10.0.0.11
    Port: 22
    User: deploy
    Password: deploy-password

Executor:
  Script: ./scripts/ssh_executor.py      # 命令执行脚本
  UploadScript: ./scripts/ssh_uploader.py # 文件上传脚本
  Concurrency: 3                          # 最多同时连接的服务器数量
  TimeoutSeconds: 120                     # 单次任务超时

Celery:
  Broker: redis://127.0.0.1:6379/0       # Celery Broker（Redis）
  Backend: redis://127.0.0.1:6379/0      # Celery Backend（Redis）
  TaskName: tasks.execute_ssh            # 命令执行任务名称
  UploadTaskName: tasks.upload_file      # 文件上传任务名称
  Workers: 2                              # Worker 并发数
```

> **提示**：
> - 所有连接信息均采用用户名 + 密码认证；请根据实际情况修改。
> - 配置文件中的 `Bastion` 和 `Targets` 是可选配置，也可以在 API 请求中动态指定。
> - 如果请求中提供了 `proxy_host` 等信息，会优先使用请求中的配置。

### 快速执行步骤

1. **安装依赖**：
   ```bash
   # Python 依赖
   python3 -m venv .venv
   source .venv/bin/activate
   pip install paramiko
   
   # Go 依赖
   go mod tidy
   ```

2. **启动 Redis**：
   ```bash
   # 使用 Docker（推荐）
   docker run -d --name redis -p 6379:6379 redis:latest
   
   # 或使用系统服务
   sudo systemctl start redis
   ```

3. **准备配置**：编辑 `etc/gocerery-api.yaml`，填入 Redis 连接信息（跳板机和目标服务器可在请求中动态指定）。

4. **启动 HTTP API**（生产者）：
   ```bash
   go run gocerery.go -f etc/gocerery-api.yaml
   ```
   看到输出 `Starting server at 0.0.0.0:8888...` 表示启动成功。

5. **启动 Celery Worker**（消费者）：
   ```bash
   # 新开一个终端窗口
   cd /home/lucien/goproject/AIDOC/gocerery-paramiko
   source .venv/bin/activate  # 如果使用虚拟环境
   go run cmd/worker/main.go -f etc/gocerery-api.yaml
   ```
   看到输出 `[WORKER] worker is running, press Ctrl+C to stop` 表示启动成功。

   > **重要**：两个进程都需要保持运行，且必须指向同一个配置文件。

6. **提交任务**：向 `POST /api/ssh/task` 或 `POST /api/upload/task` 发送 JSON（示例见下文）。

7. **查询结果**：轮询 `GET /api/ssh/task/{task_id}` 或 `GET /api/upload/task/{task_id}`；任务完成后即可看到每台机器的执行结果。


### 调用示例

```bash
curl -X POST http://localhost:8888/api/ssh/task \
  -H "Content-Type: application/json" \
  -d '{
        "proxy_host": "111.200.213.14",
        "proxy_port": 63525,
        "proxy_user": "h3c",
        "proxy_password": "******",
        "targets": [
          {
            "name": "web-1",
            "host": "172.171.2.133",
            "port": 22,
            "user": "infrawaves",
            "password": "******"
          },
          {
            "name": "web-2",
            "host": "172.171.2.134",
            "port": 22,
            "user": "infrawaves",
            "password": "******"
          }
        ],
        "commands": [
          "uname -a",
          "bash /home/infrawaves/test/test.sh"
        ],
        "timeout": 90
      }'
```

响应示例：

```json
{
  "task_id": "c146c8d5-9091-42e5-a1d6-511105db1c3b",
  "status": "PENDING",
  "message": "task submitted"
}
```

随后可以查询任务状态：

```bash
curl http://localhost:8888/api/ssh/task/c146c8d5-9091-42e5-a1d6-511105db1c3b
```

状态示例：

```json
{
  "task_id": "c146c8d5-9091-42e5-a1d6-511105db1c3b",
  "status": "SUCCESS",
  "results": [
    {
      "name": "web-1",
      "host": "172.171.2.133",
      "success": true,
      "stdout": "Linux ...",
      "stderr": "",
      "exit_code": 0
    },
    {
      "name": "web-2",
      "host": "172.171.2.134",
      "success": false,
      "stdout": "",
      "stderr": "bash: line 1: test.sh: No such file or directory",
      "exit_code": 127,
      "error": "CommandError: ..."
    }
  ]
}
```

若某台机器执行失败，其 `success` 为 `false` 且 `error` 字段包含失败原因，其它机器的结果不会受影响。

### 文件上传示例

```bash
curl -X POST http://localhost:8888/api/upload/task \
  -H "Content-Type: application/json" \
  -d '{
        "proxy_host": "111.200.213.14",
        "proxy_port": 63525,
        "proxy_user": "h3c",
        "proxy_password": "******",
        "targets": [
          {
            "name": "web-1",
            "host": "172.171.2.133",
            "port": 22,
            "user": "infrawaves",
            "password": "******"
          },
          {
            "name": "web-2",
            "host": "172.171.2.134",
            "port": 22,
            "user": "infrawaves",
            "password": "******"
          }
        ],
        "local_path": "/path/to/local/file_or_directory",
        "remote_path": "/home/infrawaves/uploaded",
        "timeout": 300
      }'
```

响应示例：

```json
{
  "task_id": "a1b2c3d4-5678-90ef-ghij-klmnopqrstuv",
  "status": "PENDING",
  "message": "upload task submitted"
}
```

查询上传任务状态：

```bash
curl http://localhost:8888/api/upload/task/a1b2c3d4-5678-90ef-ghij-klmnopqrstuv
```

状态示例：

```json
{
  "task_id": "a1b2c3d4-5678-90ef-ghij-klmnopqrstuv",
  "status": "SUCCESS",
  "results": [
    {
      "name": "web-1",
      "host": "172.171.2.133",
      "success": true,
      "uploaded_files": [
        "/home/infrawaves/uploaded/file1.txt",
        "/home/infrawaves/uploaded/subdir/file2.txt"
      ],
      "failed_files": []
    },
    {
      "name": "web-2",
      "host": "172.171.2.134",
      "success": false,
      "uploaded_files": [],
      "failed_files": [
        "/home/infrawaves/uploaded/file1.txt"
      ],
      "error": "Permission denied"
    }
  ]
}
```

**文件上传说明**：
- `local_path` 可以是文件或目录路径
- 如果是目录，会递归上传目录下的所有文件，保持目录结构
- `remote_path` 必须是远程服务器上的目录路径
- 如果远程目录不存在，会自动创建
- `uploaded_files` 包含成功上传的文件列表（远程路径）
- `failed_files` 包含上传失败的文件列表（远程路径）及错误信息

### 工作流程

**命令执行/文件上传流程**：
1. 客户端 → HTTP API → Redis 队列 → Worker → Python 脚本 → 跳板机 → 目标服务器
2. 执行结果 → Worker → Redis Backend → HTTP API → 客户端

详细流程说明请参考 [API_IMPLEMENTATION.md](./API_IMPLEMENTATION.md)

### 代码生成和 API 文档

```bash
# 生成 Go 代码（修改 gocerery.api 后）
goctl api go -api gocerery.api -dir . -style gozero

# 生成 OpenAPI 文档
goctl api swagger --api gocerery.api --dir . --filename openapi
```


### 常见问题

1. **连接失败 / 认证失败**：
   - 确保跳板机与目标机均允许用户名+密码登录
   - 检查防火墙规则，确保端口开放
   - 确认未启用额外的 2FA 或密钥认证

2. **超时问题**：
   - 可通过请求体 `timeout` 字段覆盖配置中的默认超时时长
   - 检查网络延迟和命令执行时间
   - 对于长时间运行的命令，适当增加超时时间

3. **Paramiko 模块未找到**：
   - 确认 Python 虚拟环境已经安装 `paramiko` 包
   - 如果使用虚拟环境，确保 Worker 启动时激活了虚拟环境
   - 检查 Python 路径：`which python3`

4. **任务一直处于 PENDING 状态**：
   - 检查 Worker 是否正常运行
   - 检查 Redis 连接是否正常
   - 查看 Worker 日志是否有错误信息
   - 确认任务名称匹配（`tasks.execute_ssh` 或 `tasks.upload_file`）

5. **Worker 启动后立即退出**：
   - 检查 Redis 连接配置是否正确
   - 检查配置文件路径是否正确
   - 查看错误日志

6. **部分服务器执行失败**：
   - 检查目标服务器的网络连接
   - 验证用户名和密码是否正确
   - 查看返回结果中的 `error` 字段获取详细错误信息


### 日志系统

项目使用统一的日志方案，支持文件输出、日志轮转和级别控制。

**日志文件**：
- `logs/gocerery-api.log` - HTTP API Server 日志
- `logs/gocerery-worker.log` - Celery Worker 日志
- `logs/ssh_executor.log` - 命令执行脚本日志
- `logs/ssh_uploader.log` - 文件上传脚本日志

**查看日志**：
```bash
tail -f logs/gocerery-worker.log    # 实时查看
grep -i error logs/*.log            # 搜索错误
```

详细配置和使用说明请参考 [LOGGING.md](./LOGGING.md)

### 相关文档

- [API 实现详细说明](./API_IMPLEMENTATION.md)：完整的架构说明、代码流程和技术细节
- [日志方案说明](./LOGGING.md)：日志配置、使用和调试指南

### 许可证

本项目采用 MIT 许可证。
