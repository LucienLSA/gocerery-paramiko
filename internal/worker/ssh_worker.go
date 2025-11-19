package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strconv"

	"gocerery/internal/config"

	"github.com/gocelery/gocelery"
)

type Runner struct {
	client     *gocelery.CeleryClient
	taskName   string
	scriptPath string
	timeout    int
}

func Run(cfg *config.Config) error {
	if cfg.Celery.Broker == "" || cfg.Celery.Backend == "" {
		return errors.New("celery broker/backend not configured")
	}

	broker := gocelery.NewRedisCeleryBroker(cfg.Celery.Broker)
	backend := gocelery.NewRedisCeleryBackend(cfg.Celery.Backend)
	workers := cfg.Celery.Workers
	if workers <= 0 {
		workers = 1
	}

	client, err := gocelery.NewCeleryClient(broker, backend, workers)
	if err != nil {
		return fmt.Errorf("create celery client: %w", err)
	}

	taskName := cfg.Celery.TaskName
	if taskName == "" {
		taskName = "tasks.execute_ssh"
	}

	runner := &Runner{
		client:     client,
		taskName:   taskName,
		scriptPath: filepath.Clean(cfg.Executor.Script),
		timeout:    cfg.Executor.TimeoutSeconds,
	}

	client.Register(taskName, runner.execute)
	log.Printf("celery worker registered task=%s script=%s", taskName, runner.scriptPath)

	client.StartWorker()
	return nil
}

func (r *Runner) execute(payload map[string]interface{}) (map[string]interface{}, error) {
	task, err := parsePayload(payload)
	if err != nil {
		return nil, err
	}

	if r.scriptPath == "" {
		return nil, errors.New("executor script path is empty")
	}

	timeout := normalizeTimeout(task.Timeout, r.timeout)

	bastion := map[string]interface{}{
		"host":     task.ProxyHost,
		"port":     task.ProxyPort,
		"user":     task.ProxyUser,
		"password": task.ProxyPassword,
	}
	targets := []map[string]interface{}{
		{
			"name":     task.TargetHost,
			"host":     task.TargetHost,
			"port":     task.TargetPort,
			"user":     task.TargetUser,
			"password": task.TargetPassword,
		},
	}
	commands := []string{task.Command}

	bastionJSON, _ := json.Marshal(bastion)
	targetsJSON, _ := json.Marshal(targets)
	commandsJSON, _ := json.Marshal(commands)

	args := []string{
		r.scriptPath,
		"--bastion", string(bastionJSON),
		"--targets", string(targetsJSON),
		"--commands", string(commandsJSON),
		"--concurrency", "1",
		"--timeout", strconv.Itoa(timeout),
	}

	cmd := exec.Command("python3", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return map[string]interface{}{
			"success":   false,
			"stdout":    stdout.String(),
			"stderr":    stderr.String(),
			"exit_code": 1,
			"error":     err.Error(),
		}, nil
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		return nil, fmt.Errorf("decode executor output: %w", err)
	}
	if len(results) == 0 {
		return nil, errors.New("executor returned empty result")
	}

	return results[0], nil
}

type taskPayload struct {
	ProxyHost      string
	ProxyPort      int
	ProxyUser      string
	ProxyPassword  string
	TargetHost     string
	TargetPort     int
	TargetUser     string
	TargetPassword string
	Command        string
	Timeout        int
	SaveLog        bool
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

	task := &taskPayload{
		ProxyHost:      getString("proxy_host"),
		ProxyPort:      normalizePort(getInt("proxy_port")),
		ProxyUser:      getString("proxy_user"),
		ProxyPassword:  getString("proxy_password"),
		TargetHost:     getString("target_host"),
		TargetPort:     normalizePort(getInt("target_port")),
		TargetUser:     getString("target_user"),
		TargetPassword: getString("target_password"),
		Command:        getString("command"),
		Timeout:        getInt("timeout"),
	}

	if task.ProxyHost == "" || task.ProxyUser == "" || task.ProxyPassword == "" {
		return nil, errors.New("proxy credentials are required")
	}
	if task.TargetHost == "" || task.TargetUser == "" || task.TargetPassword == "" {
		return nil, errors.New("target credentials are required")
	}
	if task.Command == "" {
		return nil, errors.New("command cannot be empty")
	}

	return task, nil
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
