// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"errors"
	"fmt"

	"gocerery/internal/svc"
	"gocerery/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ExecuteSshTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExecuteSshTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExecuteSshTaskLogic {
	return &ExecuteSshTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ExecuteSshTaskLogic) ExecuteSshTask(req *types.SshTaskRequest) (*types.SshTaskResponse, error) {
	if l.svcCtx.CeleryClient == nil {
		return nil, errors.New("celery client is not configured")
	}
	if err := validateTaskRequest(req); err != nil {
		return nil, err
	}

	taskName := l.svcCtx.Config.Celery.TaskName
	if taskName == "" {
		taskName = "tasks.execute_ssh"
	}

	targets, err := buildTargetPayloads(req.Targets)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"proxy_host":     req.ProxyHost,
		"proxy_port":     normalizePort(req.ProxyPort),
		"proxy_user":     req.ProxyUser,
		"proxy_password": req.ProxyPassword,
		"targets":        targets,
		"commands":       req.Commands,
		"timeout":        normalizeTimeout(req.Timeout, l.svcCtx.Config.Executor.TimeoutSeconds),
		"save_log":       req.SaveLog,
	}

	asyncResult, err := l.svcCtx.CeleryClient.DelayKwargs(taskName, payload)
	if err != nil {
		return nil, fmt.Errorf("submit task to celery: %w", err)
	}

	l.Logger.Infof("submitted ssh task %s for %d targets", asyncResult.TaskID, len(targets))

	return &types.SshTaskResponse{
		TaskID:  asyncResult.TaskID,
		Status:  "PENDING",
		Message: "task submitted",
	}, nil
}

func validateTaskRequest(req *types.SshTaskRequest) error {
	switch {
	case req.ProxyHost == "" || req.ProxyUser == "" || req.ProxyPassword == "":
		return errors.New("proxy host/user/password are required")
	case len(req.Targets) == 0:
		return errors.New("targets cannot be empty")
	case len(req.Commands) == 0:
		return errors.New("commands cannot be empty")
	}
	for idx, t := range req.Targets {
		if t.Host == "" || t.User == "" || t.Password == "" {
			return fmt.Errorf("target[%d] host/user/password are required", idx)
		}
	}
	return nil
}

func buildTargetPayloads(targets []types.TargetCredential) ([]map[string]interface{}, error) {
	payloads := make([]map[string]interface{}, 0, len(targets))
	for _, t := range targets {
		payloads = append(payloads, map[string]interface{}{
			"name":     t.Name,
			"host":     t.Host,
			"port":     normalizePort(t.Port),
			"user":     t.User,
			"password": t.Password,
		})
	}
	return payloads, nil
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
