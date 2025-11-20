# 项目审查与学习指南

## 一、需要改进的点

1. **Worker 未在脚本失败时返回错误（已修复）**
   - 位置：`internal/worker/ssh_worker.go`，脚本运行失败现在会直接返回 `error`，Celery 会标记为 `FAILURE`，查询接口可获取真实状态。

2. **ServiceContext 初始化直接 `log.Fatalf`（已修复）**
   - 位置：`internal/svc/servicecontext.go`，现在返回错误并由 `gocerery.go` 处理，避免 REST 服务直接崩溃。

3. **配置中硬编码敏感信息（已修复）**
   - 位置：`etc/gocerery-api.yaml` 改为 `${ENV:default}` 占位符，README 新增环境变量注入示例，仓库不再保存明文密码。

4. **Python 解释器路径固定为 `python3`**
   - 位置：`internal/worker/ssh_worker.go`（`exec.Command("python3", ...)`）。
   - 建议：在配置中增加 `Executor.PythonBin`，允许指向 `.venv/bin/python` 或容器内特定路径，降低环境差异导致的失败。

5. **API 层缺少限流/鉴权**
   - 当前任何人都可提交任务，且一次请求可下发任意指令，风险极高。
   - 建议：依托 go-zero 的 `middleware` 提供鉴权、请求来源校验，必要时记录审计日志。

## 二、学习重点

| 主题 | 关键文件 | 关注点 |
| ---- | -------- | ------ |
| go-zero 接口链路 | `gocerery.go`、`internal/handler/`、`internal/logic/`、`internal/svc/` | 理解 `RestConf`、Handler→Logic→ServiceContext 的调用方式、`goctl` 生成代码的结构 |
| gocelery 使用 | `internal/logic/*`、`internal/svc/servicecontext.go` | 学会如何通过 `DelayKwargs` 投递任务、如何查询结果、Celery 客户端初始化流程 |
| Worker 与 Python 脚本联动 | `internal/worker/ssh_worker.go`、`scripts/ssh_executor.py`、`scripts/ssh_uploader.py` | 熟悉 `exec.Command` 传参、JSON 编解码、并发线程模型以及 Paramiko 跳板机链路 |
| 日志体系 | `internal/logger/logger.go`、`LOGGING.md` | 掌握 go-zero `logx` 配置、Worker/Python 日志串联方式、如何调整日志级别与输出路径 |
| 配置与安全 | `etc/gocerery-api.yaml`、`internal/config/config.go` | 学习如何拆分配置（RestConf、WorkerLog 等），以及为何要通过环境变量管理密钥 |

建议按照以上表格线索逐步阅读代码与文档，结合真实任务（提交 SSH / 上传任务、查询结果、查看日志）进行实验，有助于理解整套链路。

