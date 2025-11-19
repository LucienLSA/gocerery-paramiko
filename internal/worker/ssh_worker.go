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
	client      *gocelery.CeleryClient
	taskName    string
	scriptPath  string
	timeout     int
	concurrency int
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
		client:      client,
		taskName:    taskName,
		scriptPath:  filepath.Clean(cfg.Executor.Script),
		timeout:     cfg.Executor.TimeoutSeconds,
		concurrency: cfg.Executor.Concurrency,
	}

	client.Register(taskName, runner.execute)
	log.Printf("celery worker registered task=%s script=%s", taskName, runner.scriptPath)

	client.StartWorker()
	return nil
}

func (r *Runner) execute(payload map[string]interface{}) (interface{}, error) {
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

	cmd := exec.Command("python3", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
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

	var results []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		return nil, fmt.Errorf("decode executor output: %w", err)
	}
	if len(results) == 0 {
		return nil, errors.New("executor returned empty result")
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
