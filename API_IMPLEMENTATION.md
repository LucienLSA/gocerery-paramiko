# API 功能实现总结

## 一、整体架构

本项目采用 **go-zero + gocelery + Paramiko** 的异步任务处理架构，实现了通过跳板机向多台服务器执行命令和上传文件的功能。

```
┌─────────────┐
│  客户端     │
└──────┬──────┘
       │ HTTP Request
       ▼
┌─────────────────────────────────┐
│  HTTP API Server (go-zero)      │
│  - Handler 层：路由处理          │
│  - Logic 层：业务逻辑            │
│  - ServiceContext：共享资源      │
└──────┬──────────────────────────┘
       │ 提交任务到队列
       ▼
┌─────────────────────────────────┐
│  Redis (Celery Broker/Backend)  │
│  - Broker：任务队列              │
│  - Backend：结果存储             │
└──────┬──────────────────────────┘
       │ Worker 消费任务
       ▼
┌─────────────────────────────────┐
│  Celery Worker (Go)             │
│  - 注册任务处理器                │
│  - 调用 Python 脚本              │
└──────┬──────────────────────────┘
       │ exec.Command
       ▼
┌─────────────────────────────────┐
│  Python Scripts (Paramiko)      │
│  - ssh_executor.py：执行命令      │
│  - ssh_uploader.py：上传文件      │
└──────┬──────────────────────────┘
       │ SSH 连接
       ▼
┌─────────────────────────────────┐
│  跳板机 (Bastion)               │
│  └─> 目标服务器1, 2, 3...        │
└─────────────────────────────────┘
```

## 二、代码生成（goctl）

使用 `goctl api go -api gocerery.api -dir . -style gozero` 生成基础代码。

**生成的文件**：
- `internal/types/types.go` - 请求/响应类型（自动生成）
- `internal/handler/routes.go` - 路由注册（自动生成）
- `handler` 和 `logic` 层需要手动实现业务逻辑

## 三、请求处理流程

### 3.1 命令执行任务流程

#### 步骤 1：HTTP 请求接收
```
POST /api/ssh/task
{
  "proxy_host": "...",
  "targets": [...],
  "commands": [...]
}
```

#### 步骤 2：Handler 层
- 解析 HTTP 请求（JSON → Go struct）
- 调用 Logic 层处理业务
- 返回 HTTP 响应（Go struct → JSON）

#### 步骤 3：Logic 层
- 验证请求参数（proxy、targets、commands 等）
- 构建 Celery 任务 payload
- 通过 `CeleryClient.DelayKwargs()` 提交任务到 Redis
- 立即返回 task_id，不等待执行完成

#### 步骤 4：ServiceContext
- 初始化 Celery 客户端（连接 Redis）
- 提供全局共享资源（Config、CeleryClient、CeleryBackend）

### 3.2 Worker 任务处理流程

#### 步骤 1：Worker 启动
- 加载配置文件
- 连接 Redis（Broker 和 Backend）
- 初始化日志系统

#### 步骤 2：Worker 初始化
- 创建 Celery 客户端
- 注册任务处理器（`tasks.execute_ssh`、`tasks.upload_file`）
- 启动 Worker，阻塞监听 Redis 队列

**关键点**：
- Worker 是独立进程，与 API Server 分离
- 通过 `client.Register()` 注册任务名称和处理器
- `StartWorkerWithContext()` 会阻塞，持续监听 Redis 队列

#### 步骤 3：任务执行
- `SshTask` 和 `UploadTask` 实现 `gocelery.CeleryTask` 接口
- `ParseKwargs()` 解析任务参数（因为使用 `DelayKwargs()` 发送 kwargs）
- `RunTask()` 执行实际任务逻辑

#### 步骤 4：执行 Python 脚本
- 解析任务 payload
- 构建 Python 脚本命令行参数（JSON 通过参数传递）
- 使用 `exec.Command()` 执行 Python 脚本
- 捕获 stdout/stderr，解析 JSON 结果
- 返回结果，gocelery 自动存储到 Redis Backend

#### 步骤 5：Python 脚本执行
- 解析命令行参数（bastion、targets、commands）
- 使用 Paramiko 连接跳板机，通过 `direct-tcpip` 通道连接目标服务器
- 多线程并发处理多个目标服务器
- 执行命令，收集 stdout/stderr/exit_code
- 输出 JSON 格式结果到 stdout

### 3.3 查询任务结果流程

1. **客户端** → `GET /api/ssh/task/{task_id}` → **HTTP API Server**
2. **Handler 层** → 从 URL path 解析 task_id
3. **Logic 层** → 使用 `CeleryBackend.GetResult()` 从 Redis 查询结果
4. **返回结果** → 如果任务未完成返回 `PENDING`，完成则返回详细结果

## 四、文件上传功能实现

文件上传功能的实现流程与命令执行类似，主要区别：

### 4.1 API 接口
- `POST /api/upload/task` - 提交上传任务
- `GET /api/upload/task/:id` - 查询上传任务状态

### 4.2 任务类型
- 任务名称：`tasks.upload_file`（配置中的 `UploadTaskName`）
- Python 脚本：`scripts/ssh_uploader.py`

### 4.3 请求参数
```json
{
  "proxy_host": "...",
  "targets": [...],
  "local_path": "/path/to/local/file_or_directory",
  "remote_path": "/path/to/remote/directory"
}
```

### 4.4 Python 脚本实现
- 连接跳板机 → 目标服务器（与命令执行相同）
- 使用 SFTP 上传文件
- 支持单文件和目录递归上传
- 返回上传结果（成功/失败文件列表）

## 五、关键技术点

### 5.1 异步任务处理
使用 gocelery 实现异步任务队列：API Server 立即返回 task_id，Worker 后台处理，客户端通过 task_id 轮询结果。

### 5.2 跳板机连接
使用 Paramiko 的 `direct-tcpip` 通道实现跳板机转发：
```python
transport = bastion_client.get_transport()
channel = transport.open_channel("direct-tcpip", dest_addr, local_addr)
target_client.connect(sock=channel)
```

### 5.3 并发处理
- Go Worker 层：配置 `Concurrency` 参数
- Python 脚本层：使用多线程并发执行

### 5.4 结果存储
使用 Redis 作为 Celery Backend：Worker 执行完成后结果自动存储到 Redis，API Server 通过 `CeleryBackend.GetResult()` 查询。

### 5.5 错误处理
每台服务器独立处理，收集各自的错误，返回结果包含每台服务器的 `success` 状态和 `error` 信息。

## 六、配置说明

配置文件 `etc/gocerery-api.yaml` 包含：
- HTTP 服务配置（Name、Host、Port）
- 跳板机和目标服务器配置（可选，也可在请求中动态指定）
- 执行器配置（脚本路径、并发数、超时）
- Celery 配置（Broker、Backend、任务名称、Worker 数量）
- 日志配置（模式、路径、级别等）

**配置优先级**：请求参数 > 配置文件

## 七、启动流程

### 7.1 启动 HTTP API Server
```bash
go run gocerery.go -f etc/gocerery-api.yaml
```
加载配置 → 初始化 ServiceContext（连接 Redis）→ 注册路由 → 启动 HTTP 服务器

### 7.2 启动 Celery Worker
```bash
go run cmd/worker/main.go -f etc/gocerery-api.yaml
```
加载配置 → 连接 Redis → 注册任务处理器 → 启动 Worker（阻塞监听）

### 7.3 两个进程的关系
- **API Server**：处理 HTTP 请求，提交任务到队列
- **Worker**：消费队列任务，执行实际 SSH 操作
- **Redis**：作为中间件，连接 API Server 和 Worker

## 八、总结

### 架构优势
- **异步处理**：API 响应快速，不阻塞
- **解耦设计**：API Server 和 Worker 独立部署
- **可扩展性**：可以启动多个 Worker 提高处理能力
- **可靠性**：任务结果持久化到 Redis

### 技术栈
- **go-zero**：HTTP 框架和代码生成
- **gocelery**：Go 实现的 Celery 客户端/Worker
- **Paramiko**：Python SSH 库
- **Redis**：消息队列和结果存储

### 数据流
```
客户端 → HTTP API → Redis Queue → Worker → Python Script → 跳板机 → 目标服务器
                                                                    ↓
客户端 ← HTTP API ← Redis Backend ← Worker ← Python Script ← 执行结果
```

