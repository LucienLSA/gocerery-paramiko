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

- `gocerery.go`：HTTP 服务入口
- `etc/gocerery-api.yaml`：服务及 SSH 相关配置
- `scripts/ssh_executor.py`：通过 Paramiko 执行命令的脚本
- `scripts/ssh_uploader.py`：通过 Paramiko 上传文件的脚本
- `internal/`：go-zero 生成的服务代码

### 运行依赖

- Go 1.20+
- Python 3.8+
- Redis（作为 Celery Broker + Backend）
- pip 安装 `paramiko`

```bash
cd /home/lucien/goproject/AIDOC/gocerery-paramiko
python3 -m venv .venv
source .venv/bin/activate
pip install -r <(printf "paramiko\n")
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

> **提示**：所有连接信息均采用用户名 + 密码认证；请根据实际情况修改。

### 快速执行步骤

1. **安装依赖**：按照前述命令创建 Python 虚拟环境并安装 `paramiko`，同时 `go mod tidy`。
2. **准备配置**：编辑 `etc/gocerery-api.yaml`，填入跳板机、目标服务器、Redis 等信息。
3. **启动 HTTP API**（生产者）：
   ```bash
   go run gocerery.go -f etc/gocerery-api.yaml
   ```
4. **启动 Celery Worker**（消费者 + Paramiko）：
   ```bash
   go run cmd/worker/main.go -f etc/gocerery-api.yaml
   ```
   > 两个进程都需要保持运行，且必须指向同一个配置文件。
5. **提交任务**：向 `POST /api/ssh/task` 发送 JSON（示例见下文）。
6. **查询结果**：轮询 `GET /api/ssh/task/{task_id}`；任务完成后即可看到每台机器的 stdout/stderr/exit_code。

### 启动 HTTP 服务

```bash
cd /home/lucien/goproject/AIDOC/gocerery-paramiko
go mod tidy
go run gocerery.go -f etc/gocerery-api.yaml
```

### 启动 Celery Worker

Worker 连接同一份配置，消费任务并调用 `paramiko` 脚本：

```bash
cd /home/lucien/goproject/AIDOC/gocerery-paramiko
go run cmd/worker/main.go -f etc/gocerery-api.yaml
```

> Worker 会自动注册 `tasks.execute_ssh` 和 `tasks.upload_file` 任务，分别调用 `scripts/ssh_executor.py` 和 `scripts/ssh_uploader.py` 执行真实的 SSH 操作。

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

### 常见问题

1. **连接失败 / 认证失败**：确保跳板机与目标机均允许用户名+密码登录，并且未启用额外的 2FA。
2. **超时**：可通过请求体 `timeout_sec` 字段覆盖配置中的默认超时时长。
3. **Paramiko 模块未找到**：确认 Python 虚拟环境已经安装 `paramiko` 包，或者在系统级环境中安装。
