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

type ExecuteUploadTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExecuteUploadTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExecuteUploadTaskLogic {
	return &ExecuteUploadTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ExecuteUploadTaskLogic) ExecuteUploadTask(req *types.UploadTaskRequest) (*types.UploadTaskResponse, error) {
	if l.svcCtx.CeleryClient == nil {
		return nil, errors.New("celery client is not configured")
	}
	if err := validateUploadTaskRequest(req); err != nil {
		return nil, err
	}

	taskName := "tasks.upload_file"
	if l.svcCtx.Config.Celery.UploadTaskName != "" {
		taskName = l.svcCtx.Config.Celery.UploadTaskName
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
		"local_path":     req.LocalPath,
		"remote_path":    req.RemotePath,
		"timeout":        normalizeTimeout(req.Timeout, l.svcCtx.Config.Executor.TimeoutSeconds),
		"save_log":       req.SaveLog,
	}

	asyncResult, err := l.svcCtx.CeleryClient.DelayKwargs(taskName, payload)
	if err != nil {
		return nil, fmt.Errorf("submit upload task to celery: %w", err)
	}

	l.Logger.Infof("submitted upload task %s for %d targets", asyncResult.TaskID, len(targets))

	return &types.UploadTaskResponse{
		TaskID:  asyncResult.TaskID,
		Status:  "PENDING",
		Message: "upload task submitted",
	}, nil
}

func validateUploadTaskRequest(req *types.UploadTaskRequest) error {
	switch {
	case req.ProxyHost == "" || req.ProxyUser == "" || req.ProxyPassword == "":
		return errors.New("proxy host/user/password are required")
	case len(req.Targets) == 0:
		return errors.New("targets cannot be empty")
	case req.LocalPath == "":
		return errors.New("local_path is required")
	case req.RemotePath == "":
		return errors.New("remote_path is required")
	}
	for idx, t := range req.Targets {
		if t.Host == "" || t.User == "" || t.Password == "" {
			return fmt.Errorf("target[%d] host/user/password are required", idx)
		}
	}
	return nil
}
