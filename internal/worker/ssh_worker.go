package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"gocerery/internal/config"

	"github.com/gocelery/gocelery"
)

type Runner struct {
	client           *gocelery.CeleryClient
	taskName         string
	uploadTaskName   string
	scriptPath       string
	uploadScriptPath string
	timeout          int
	concurrency      int
}

// SshTask 实现 CeleryTask 接口，用于处理 kwargs
type SshTask struct {
	runner  *Runner
	mu      sync.Mutex
	payload map[string]interface{}
}

// ParseKwargs 解析 kwargs 参数
func (t *SshTask) ParseKwargs(kwargs map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.payload = kwargs
	return nil
}

// RunTask 执行任务
func (t *SshTask) RunTask() (interface{}, error) {
	t.mu.Lock()
	payload := t.payload
	t.mu.Unlock()
	return t.runner.execute(payload)
}

// UploadTask 实现 CeleryTask 接口，用于处理文件上传任务
type UploadTask struct {
	runner  *Runner
	mu      sync.Mutex
	payload map[string]interface{}
}

// ParseKwargs 解析 kwargs 参数
func (t *UploadTask) ParseKwargs(kwargs map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.payload = kwargs
	return nil
}

// RunTask 执行任务
func (t *UploadTask) RunTask() (interface{}, error) {
	t.mu.Lock()
	payload := t.payload
	t.mu.Unlock()
	return t.runner.executeUpload(payload)
}

func Run(cfg *config.Config) error {
	log.Printf("[WORKER] initializing Celery worker...")
	log.Printf("[WORKER] initializing Celery worker...")
	if cfg.Celery.Broker == "" || cfg.Celery.Backend == "" {
		return errors.New("celery broker/backend not configured")
	}

	log.Printf("[WORKER] connecting to broker: %s", cfg.Celery.Broker)
	log.Printf("[WORKER] connecting to backend: %s", cfg.Celery.Backend)
	log.Printf("[WORKER] connecting to broker: %s", cfg.Celery.Broker)
	log.Printf("[WORKER] connecting to backend: %s", cfg.Celery.Backend)
	broker := gocelery.NewRedisCeleryBroker(cfg.Celery.Broker)
	backend := gocelery.NewRedisCeleryBackend(cfg.Celery.Backend)
	workers := cfg.Celery.Workers
	if workers <= 0 {
		workers = 1
	}

	log.Printf("[WORKER] creating Celery client with %d workers...", workers)
	log.Printf("[WORKER] creating Celery client with %d workers...", workers)
	client, err := gocelery.NewCeleryClient(broker, backend, workers)
	if err != nil {
		return fmt.Errorf("create celery client: %w", err)
	}

	taskName := cfg.Celery.TaskName
	if taskName == "" {
		taskName = "tasks.execute_ssh"
	}

	scriptPath := filepath.Clean(cfg.Executor.Script)
	uploadScriptPath := filepath.Clean(cfg.Executor.UploadScript)
	if uploadScriptPath == "" {
		uploadScriptPath = "./scripts/ssh_uploader.py"
	}

	uploadTaskName := cfg.Celery.UploadTaskName
	if uploadTaskName == "" {
		uploadTaskName = "tasks.upload_file"
	}

	runner := &Runner{
		client:           client,
		taskName:         taskName,
		uploadTaskName:   uploadTaskName,
		scriptPath:       scriptPath,
		uploadScriptPath: uploadScriptPath,
		timeout:          cfg.Executor.TimeoutSeconds,
		concurrency:      cfg.Executor.Concurrency,
	}

	log.Printf("[WORKER] registering task: %s", taskName)
	// 创建并注册实现了 CeleryTask 接口的任务对象
	sshTask := &SshTask{runner: runner}
	client.Register(taskName, sshTask)

	log.Printf("[WORKER] registering upload task: %s", uploadTaskName)
	uploadTask := &UploadTask{runner: runner}
	client.Register(uploadTaskName, uploadTask)

	log.Printf("[WORKER] celery worker ready: task=%s script=%s upload_task=%s upload_script=%s timeout=%ds concurrency=%d",
		taskName, scriptPath, uploadTaskName, uploadScriptPath, runner.timeout, runner.concurrency)

	// 创建 context 用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置信号处理，优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("[WORKER] received signal: %v, shutting down...", sig)
		cancel()
		client.StopWorker()
		log.Printf("[WORKER] worker stopped")
	}()

	log.Printf("[WORKER] starting worker, waiting for tasks...")
	log.Printf("[WORKER] worker is running, press Ctrl+C to stop")

	// 使用 context 启动 worker（阻塞调用）
	client.StartWorkerWithContext(ctx)

	// 等待 worker 完全停止
	client.WaitForStopWorker()
	log.Printf("[WORKER] worker exited")
	return nil
}

func (r *Runner) execute(payload map[string]interface{}) (interface{}, error) {
	log.Printf("[WORKER] received task, parsing payload...")
	log.Printf("[WORKER] received task, parsing payload...")
	task, err := parsePayload(payload)
	if err != nil {
		log.Printf("[WORKER] failed to parse payload: %v", err)
		log.Printf("[WORKER] failed to parse payload: %v", err)
		return nil, err
	}

	log.Printf("[WORKER] task parsed: proxy=%s:%d, targets=%d, commands=%d",
		task.ProxyHost, task.ProxyPort, len(task.Targets), len(task.Commands))

	log.Printf("[WORKER] task parsed: proxy=%s:%d, targets=%d, commands=%d",
		task.ProxyHost, task.ProxyPort, len(task.Targets), len(task.Commands))

	if r.scriptPath == "" {
		log.Printf("[WORKER] executor script path is empty")
		log.Printf("[WORKER] executor script path is empty")
		return nil, errors.New("executor script path is empty")
	}

	timeout := normalizeTimeout(task.Timeout, r.timeout)
	log.Printf("[WORKER] using timeout=%ds, concurrency=%d", timeout, r.concurrency)
	log.Printf("[WORKER] using timeout=%ds, concurrency=%d", timeout, r.concurrency)

	bastion := map[string]interface{}{
		"host":     task.ProxyHost,
		"port":     task.ProxyPort,
		"user":     task.ProxyUser,
		"password": task.ProxyPassword,
	}
	targets := make([]map[string]interface{}, 0, len(task.Targets))
	for _, t := range task.Targets {
		targets = append(targets, map[string]interface{}{
			"name":     t.Name,
			"host":     t.Host,
			"port":     t.Port,
			"user":     t.User,
			"password": t.Password,
		})
	}

	bastionJSON, _ := json.Marshal(bastion)
	targetsJSON, _ := json.Marshal(targets)
	commandsJSON, _ := json.Marshal(task.Commands)

	concurrency := r.concurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(targets) && len(targets) > 0 {
		concurrency = len(targets)
	}

	args := []string{
		r.scriptPath,
		"--bastion", string(bastionJSON),
		"--targets", string(targetsJSON),
		"--commands", string(commandsJSON),
		"--concurrency", strconv.Itoa(concurrency),
		"--timeout", strconv.Itoa(timeout),
	}

	log.Printf("[WORKER] executing script: python3 %s", r.scriptPath)
	for i, target := range task.Targets {
		log.Printf("[WORKER] target[%d]: %s (%s:%d)", i, target.Name, target.Host, target.Port)
	}
	for i, cmd := range task.Commands {
		log.Printf("[WORKER] command[%d]: %s", i, cmd)
	}

	log.Printf("[WORKER] executing script: python3 %s", r.scriptPath)
	for i, target := range task.Targets {
		log.Printf("[WORKER] target[%d]: %s (%s:%d)", i, target.Name, target.Host, target.Port)
	}
	for i, cmd := range task.Commands {
		log.Printf("[WORKER] command[%d]: %s", i, cmd)
	}

	cmd := exec.Command("python3", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("[WORKER] starting script execution...")
	log.Printf("[WORKER] starting script execution...")
	if err := cmd.Run(); err != nil {
		log.Printf("[WORKER] script execution failed: %v", err)
		log.Printf("[WORKER] script stderr: %s", stderr.String())
		log.Printf("[WORKER] script execution failed: %v", err)
		log.Printf("[WORKER] script stderr: %s", stderr.String())
		return []map[string]interface{}{
			{
				"name":      "",
				"host":      "",
				"success":   false,
				"stdout":    stdout.String(),
				"stderr":    stderr.String(),
				"exit_code": 1,
				"error":     err.Error(),
			},
		}, nil
	}
	log.Printf("[WORKER] script execution completed, stdout length=%d", stdout.Len())
	if stdout.Len() > 0 {
		// 打印原始输出（前 500 个字符）用于调试
		stdoutStr := stdout.String()
		if len(stdoutStr) > 500 {
			log.Printf("[WORKER] raw stdout (truncated): %s...", stdoutStr[:500])
		} else {
			log.Printf("[WORKER] raw stdout: %s", stdoutStr)
		}
	}
	if stderr.Len() > 0 {
		log.Printf("[WORKER] script stderr: %s", stderr.String())
	}
	log.Printf("[WORKER] script execution completed, stdout length=%d", stdout.Len())
	if stdout.Len() > 0 {
		// 打印原始输出（前 500 个字符）用于调试
		stdoutStr := stdout.String()
		if len(stdoutStr) > 500 {
			log.Printf("[WORKER] raw stdout (truncated): %s...", stdoutStr[:500])
		} else {
			log.Printf("[WORKER] raw stdout: %s", stdoutStr)
		}
	}
	if stderr.Len() > 0 {
		log.Printf("[WORKER] script stderr: %s", stderr.String())
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		log.Printf("[WORKER] failed to decode executor output: %v", err)
		log.Printf("[WORKER] raw stdout: %s", stdout.String())
		log.Printf("[WORKER] failed to decode executor output: %v", err)
		log.Printf("[WORKER] raw stdout: %s", stdout.String())
		return nil, fmt.Errorf("decode executor output: %w", err)
	}
	if len(results) == 0 {
		log.Printf("[WORKER] executor returned empty result")
		log.Printf("[WORKER] executor returned empty result")
		return nil, errors.New("executor returned empty result")
	}

	successCount := 0
	for _, r := range results {
		if success, ok := r["success"].(bool); ok && success {
			successCount++
		}
	}
	log.Printf("[WORKER] task completed: %d/%d targets succeeded", successCount, len(results))
	for i, r := range results {
		name := "unknown"
		if n, ok := r["name"].(string); ok {
			name = n
		}
		host := "unknown"
		if h, ok := r["host"].(string); ok {
			host = h
		}
		success := false
		if s, ok := r["success"].(bool); ok {
			success = s
		}
		exitCode := 0
		if ec, ok := r["exit_code"].(float64); ok {
			exitCode = int(ec)
		} else if ec, ok := r["exit_code"].(int); ok {
			exitCode = ec
		}
		stdout := ""
		if so, ok := r["stdout"].(string); ok {
			stdout = so
		}
		stderr := ""
		if se, ok := r["stderr"].(string); ok {
			stderr = se
		}
		errMsg := ""
		if e, ok := r["error"].(string); ok {
			errMsg = e
		}
		log.Printf("[WORKER] result[%d]: %s (%s) success=%v exit_code=%d", i, name, host, success, exitCode)
		if !success {
			if errMsg != "" {
				log.Printf("[WORKER] result[%d] error: %s", i, errMsg)
			}
			if stderr != "" {
				log.Printf("[WORKER] result[%d] stderr: %s", i, stderr)
			}
		}
		if stdout != "" {
			// 只显示前 200 个字符，避免日志过长
			if len(stdout) > 200 {
				log.Printf("[WORKER] result[%d] stdout (truncated): %s...", i, stdout[:200])
			} else {
				log.Printf("[WORKER] result[%d] stdout: %s", i, stdout)
			}
		}
	}

	return results, nil
}

func (r *Runner) executeUpload(payload map[string]interface{}) (interface{}, error) {
	log.Printf("[WORKER] received upload task, parsing payload...")
	task, err := parseUploadPayload(payload)
	if err != nil {
		log.Printf("[WORKER] failed to parse upload payload: %v", err)
		return nil, err
	}

	log.Printf("[WORKER] upload task parsed: proxy=%s:%d, targets=%d, local_path=%s, remote_path=%s",
		task.ProxyHost, task.ProxyPort, len(task.Targets), task.LocalPath, task.RemotePath)

	if r.uploadScriptPath == "" {
		log.Printf("[WORKER] upload script path is empty")
		return nil, errors.New("upload script path is empty")
	}

	// 检查本地路径是否存在
	if _, err := os.Stat(task.LocalPath); os.IsNotExist(err) {
		log.Printf("[WORKER] local path does not exist: %s", task.LocalPath)
		return nil, fmt.Errorf("local path does not exist: %s", task.LocalPath)
	}

	timeout := normalizeTimeout(task.Timeout, r.timeout)
	log.Printf("[WORKER] using timeout=%ds, concurrency=%d", timeout, r.concurrency)

	bastion := map[string]interface{}{
		"host":     task.ProxyHost,
		"port":     task.ProxyPort,
		"user":     task.ProxyUser,
		"password": task.ProxyPassword,
	}
	targets := make([]map[string]interface{}, 0, len(task.Targets))
	for _, t := range task.Targets {
		targets = append(targets, map[string]interface{}{
			"name":     t.Name,
			"host":     t.Host,
			"port":     t.Port,
			"user":     t.User,
			"password": t.Password,
		})
	}

	bastionJSON, _ := json.Marshal(bastion)
	targetsJSON, _ := json.Marshal(targets)

	concurrency := r.concurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(targets) && len(targets) > 0 {
		concurrency = len(targets)
	}

	args := []string{
		r.uploadScriptPath,
		"--bastion", string(bastionJSON),
		"--targets", string(targetsJSON),
		"--local-path", task.LocalPath,
		"--remote-path", task.RemotePath,
		"--concurrency", strconv.Itoa(concurrency),
		"--timeout", strconv.Itoa(timeout),
	}

	log.Printf("[WORKER] executing upload script: python3 %s", r.uploadScriptPath)
	for i, target := range task.Targets {
		log.Printf("[WORKER] target[%d]: %s (%s:%d)", i, target.Name, target.Host, target.Port)
	}
	log.Printf("[WORKER] local_path: %s", task.LocalPath)
	log.Printf("[WORKER] remote_path: %s", task.RemotePath)

	cmd := exec.Command("python3", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("[WORKER] starting upload script execution...")
	if err := cmd.Run(); err != nil {
		log.Printf("[WORKER] upload script execution failed: %v", err)
		log.Printf("[WORKER] upload script stderr: %s", stderr.String())
		return []map[string]interface{}{
			{
				"name":           "",
				"host":           "",
				"success":        false,
				"uploaded_files": []string{},
				"failed_files":   []string{},
				"error":          err.Error(),
			},
		}, nil
	}
	log.Printf("[WORKER] upload script execution completed, stdout length=%d", stdout.Len())
	if stdout.Len() > 0 {
		stdoutStr := stdout.String()
		if len(stdoutStr) > 500 {
			log.Printf("[WORKER] raw stdout (truncated): %s...", stdoutStr[:500])
		} else {
			log.Printf("[WORKER] raw stdout: %s", stdoutStr)
		}
	}
	if stderr.Len() > 0 {
		log.Printf("[WORKER] upload script stderr: %s", stderr.String())
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		log.Printf("[WORKER] failed to decode upload executor output: %v", err)
		log.Printf("[WORKER] raw stdout: %s", stdout.String())
		return nil, fmt.Errorf("decode upload executor output: %w", err)
	}
	if len(results) == 0 {
		log.Printf("[WORKER] upload executor returned empty result")
		return nil, errors.New("upload executor returned empty result")
	}

	successCount := 0
	for _, r := range results {
		if success, ok := r["success"].(bool); ok && success {
			successCount++
		}
	}
	log.Printf("[WORKER] upload task completed: %d/%d targets succeeded", successCount, len(results))
	for i, r := range results {
		name := "unknown"
		if n, ok := r["name"].(string); ok {
			name = n
		}
		host := "unknown"
		if h, ok := r["host"].(string); ok {
			host = h
		}
		success := false
		if s, ok := r["success"].(bool); ok {
			success = s
		}
		uploadedFiles := []string{}
		if uf, ok := r["uploaded_files"].([]interface{}); ok {
			for _, f := range uf {
				if fileInfo, ok := f.(map[string]interface{}); ok {
					if remote, ok := fileInfo["remote"].(string); ok {
						uploadedFiles = append(uploadedFiles, remote)
					}
				}
			}
		}
		failedFiles := []string{}
		if ff, ok := r["failed_files"].([]interface{}); ok {
			for _, f := range ff {
				if fileInfo, ok := f.(map[string]interface{}); ok {
					if remote, ok := fileInfo["remote"].(string); ok {
						failedFiles = append(failedFiles, remote)
					}
				}
			}
		}
		errMsg := ""
		if e, ok := r["error"].(string); ok {
			errMsg = e
		}
		log.Printf("[WORKER] upload result[%d]: %s (%s) success=%v uploaded=%d failed=%d", i, name, host, success, len(uploadedFiles), len(failedFiles))
		if !success {
			if errMsg != "" {
				log.Printf("[WORKER] upload result[%d] error: %s", i, errMsg)
			}
		}
	}

	return results, nil
}

type taskPayload struct {
	ProxyHost     string
	ProxyPort     int
	ProxyUser     string
	ProxyPassword string
	Targets       []targetPayload
	Commands      []string
	Timeout       int
	SaveLog       bool
}

type targetPayload struct {
	Name     string
	Host     string
	Port     int
	User     string
	Password string
}

func parsePayload(data map[string]interface{}) (*taskPayload, error) {
	getString := func(key string) string {
		if v, ok := data[key]; ok {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}

	getInt := func(key string) int {
		if v, ok := data[key]; ok {
			switch val := v.(type) {
			case float64:
				return int(val)
			case int:
				return val
			case string:
				if parsed, err := strconv.Atoi(val); err == nil {
					return parsed
				}
			}
		}
		return 0
	}

	rawTargets, ok := data["targets"]
	if !ok {
		return nil, errors.New("targets is required")
	}
	targets, err := parseTargets(rawTargets)
	if err != nil {
		return nil, err
	}
	rawCommands, ok := data["commands"]
	if !ok {
		return nil, errors.New("commands is required")
	}
	commands, err := parseCommands(rawCommands)
	if err != nil {
		return nil, err
	}

	task := &taskPayload{
		ProxyHost:     getString("proxy_host"),
		ProxyPort:     normalizePort(getInt("proxy_port")),
		ProxyUser:     getString("proxy_user"),
		ProxyPassword: getString("proxy_password"),
		Targets:       targets,
		Commands:      commands,
		Timeout:       getInt("timeout"),
	}

	if task.ProxyHost == "" || task.ProxyUser == "" || task.ProxyPassword == "" {
		return nil, errors.New("proxy credentials are required")
	}
	if len(task.Targets) == 0 {
		return nil, errors.New("targets cannot be empty")
	}
	if len(task.Commands) == 0 {
		return nil, errors.New("commands cannot be empty")
	}

	return task, nil
}

func parseTargets(raw interface{}) ([]targetPayload, error) {
	targetSlice, ok := raw.([]interface{})
	if !ok {
		return nil, errors.New("targets must be an array")
	}
	result := make([]targetPayload, 0, len(targetSlice))
	for idx, item := range targetSlice {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("target[%d] must be object", idx)
		}
		name := fmt.Sprintf("%v", obj["name"])
		host := fmt.Sprintf("%v", obj["host"])
		user := fmt.Sprintf("%v", obj["user"])
		password := fmt.Sprintf("%v", obj["password"])
		port := 0
		if v, ok := obj["port"]; ok {
			switch p := v.(type) {
			case float64:
				port = int(p)
			case int:
				port = p
			case string:
				if parsed, err := strconv.Atoi(p); err == nil {
					port = parsed
				}
			}
		}
		port = normalizePort(port)
		if host == "" || user == "" || password == "" {
			return nil, fmt.Errorf("target[%d] host/user/password are required", idx)
		}
		result = append(result, targetPayload{
			Name:     name,
			Host:     host,
			Port:     port,
			User:     user,
			Password: password,
		})
	}
	return result, nil
}

func parseCommands(raw interface{}) ([]string, error) {
	switch val := raw.(type) {
	case []interface{}:
		cmds := make([]string, 0, len(val))
		for idx, item := range val {
			str := fmt.Sprintf("%v", item)
			if str == "" {
				return nil, fmt.Errorf("commands[%d] cannot be empty", idx)
			}
			cmds = append(cmds, str)
		}
		return cmds, nil
	case []string:
		return val, nil
	default:
		return nil, errors.New("commands must be an array")
	}
}

func normalizePort(port int) int {
	if port <= 0 {
		return 22
	}
	return port
}

func normalizeTimeout(requestTimeout, defaultTimeout int) int {
	if requestTimeout > 0 {
		return requestTimeout
	}
	if defaultTimeout > 0 {
		return defaultTimeout
	}
	return 120
}

type uploadTaskPayload struct {
	ProxyHost     string
	ProxyPort     int
	ProxyUser     string
	ProxyPassword string
	Targets       []targetPayload
	LocalPath     string
	RemotePath    string
	Timeout       int
	SaveLog       bool
}

func parseUploadPayload(data map[string]interface{}) (*uploadTaskPayload, error) {
	getString := func(key string) string {
		if v, ok := data[key]; ok {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}

	getInt := func(key string) int {
		if v, ok := data[key]; ok {
			switch val := v.(type) {
			case float64:
				return int(val)
			case int:
				return val
			case string:
				if parsed, err := strconv.Atoi(val); err == nil {
					return parsed
				}
			}
		}
		return 0
	}

	rawTargets, ok := data["targets"]
	if !ok {
		return nil, errors.New("targets is required")
	}
	targets, err := parseTargets(rawTargets)
	if err != nil {
		return nil, err
	}

	task := &uploadTaskPayload{
		ProxyHost:     getString("proxy_host"),
		ProxyPort:     normalizePort(getInt("proxy_port")),
		ProxyUser:     getString("proxy_user"),
		ProxyPassword: getString("proxy_password"),
		Targets:       targets,
		LocalPath:     getString("local_path"),
		RemotePath:    getString("remote_path"),
		Timeout:       getInt("timeout"),
	}

	if task.ProxyHost == "" || task.ProxyUser == "" || task.ProxyPassword == "" {
		return nil, errors.New("proxy credentials are required")
	}
	if len(task.Targets) == 0 {
		return nil, errors.New("targets cannot be empty")
	}
	if task.LocalPath == "" {
		return nil, errors.New("local_path is required")
	}
	if task.RemotePath == "" {
		return nil, errors.New("remote_path is required")
	}

	return task, nil
}
