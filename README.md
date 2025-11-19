## gocerery-paramiko

使用 go-zero + Paramiko 搭建的简易 SSH 任务分发服务。服务端提供 REST API，接收需要执行的命令列表，随后通过 Paramiko 先登录跳板机，再使用用户名/密码方式登录多台目标服务器并执行指令。

### 目录结构

- `gocerery.go`：HTTP 服务入口
- `etc/gocerery-api.yaml`：服务及 SSH 相关配置
- `scripts/ssh_executor.py`：通过 Paramiko 执行命令的脚本
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
  Script: ./scripts/ssh_executor.py   # 默认为 python 脚本，可根据需要替换
  Concurrency: 3                      # 最多同时连接的服务器数量
  TimeoutSeconds: 120                 # 单次任务超时

Celery:
  Broker: redis://127.0.0.1:6379/0     # Celery Broker（Redis）
  Backend: redis://127.0.0.1:6379/0    # Celery Backend（Redis）
  TaskName: tasks.execute_ssh          # Celery 任务名称
  Workers: 2                           # Worker 并发数
```

> **提示**：所有连接信息均采用用户名 + 密码认证；请根据实际情况修改。

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

> Worker 会自动注册 `tasks.execute_ssh` 任务，内部调用 `scripts/ssh_executor.py` 执行真实的 SSH 操作。

### 调用示例

```bash
curl -X POST http://localhost:8888/api/ssh/task \
  -H "Content-Type: application/json" \
  -d '{
        "commands": ["hostname", "uptime"],
        "targets": ["web-server"],
        "timeout_sec": 90
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
  "result": {
    "name": "172.171.2.133",
    "host": "172.171.2.133",
    "success": true,
    "stdout": "Linux ...",
    "stderr": "",
    "exit_code": 0
  }
}
```

若执行失败，`result.success` 为 `false` 且 `error` 字段包含失败原因。

### 常见问题

1. **连接失败 / 认证失败**：确保跳板机与目标机均允许用户名+密码登录，并且未启用额外的 2FA。
2. **超时**：可通过请求体 `timeout_sec` 字段覆盖配置中的默认超时时长。
3. **Paramiko 模块未找到**：确认 Python 虚拟环境已经安装 `paramiko` 包，或者在系统级环境中安装。
