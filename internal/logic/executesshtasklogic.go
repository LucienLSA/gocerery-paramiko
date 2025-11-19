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

	payload := map[string]interface{}{
		"proxy_host":      req.ProxyHost,
		"proxy_port":      normalizePort(req.ProxyPort),
		"proxy_user":      req.ProxyUser,
		"proxy_password":  req.ProxyPassword,
		"target_host":     req.TargetHost,
		"target_port":     normalizePort(req.TargetPort),
		"target_user":     req.TargetUser,
		"target_password": req.TargetPassword,
		"command":         req.Command,
		"timeout":         normalizeTimeout(req.Timeout, l.svcCtx.Config.Executor.TimeoutSeconds),
		"save_log":        req.SaveLog,
	}

	asyncResult, err := l.svcCtx.CeleryClient.DelayKwargs(taskName, payload)
	if err != nil {
		return nil, fmt.Errorf("submit task to celery: %w", err)
	}

	l.Logger.Infof("submitted ssh task %s for target %s", asyncResult.TaskID, req.TargetHost)

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
	case req.TargetHost == "" || req.TargetUser == "" || req.TargetPassword == "":
		return errors.New("target host/user/password are required")
	case req.Command == "":
		return errors.New("command cannot be empty")
	}
	return nil
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
