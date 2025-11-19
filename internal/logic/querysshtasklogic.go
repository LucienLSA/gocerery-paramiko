// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gocerery/internal/svc"
	"gocerery/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type QuerySshTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewQuerySshTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QuerySshTaskLogic {
	return &QuerySshTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QuerySshTaskLogic) QuerySshTask(req *types.SshTaskStatusRequest) (*types.SshTaskStatusResponse, error) {
	if req.TaskID == "" {
		return nil, errors.New("task_id is required")
	}
	if l.svcCtx.CeleryBackend == nil {
		return nil, errors.New("celery backend is not configured")
	}

	resultMsg, err := l.svcCtx.CeleryBackend.GetResult(req.TaskID)
	if err != nil {
		l.Logger.Infof("task %s not finished yet: %v", req.TaskID, err)
		return &types.SshTaskStatusResponse{
			TaskID: req.TaskID,
			Status: "PENDING",
			Error:  err.Error(),
		}, nil
	}

	resp := &types.SshTaskStatusResponse{
		TaskID: req.TaskID,
		Status: resultMsg.Status,
	}

	if resultMsg.Result != nil {
		resultBytes, marshalErr := json.Marshal(resultMsg.Result)
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal task result: %w", marshalErr)
		}
		var hostResult types.HostResult
		if err := json.Unmarshal(resultBytes, &hostResult); err != nil {
			return nil, fmt.Errorf("unmarshal task result: %w", err)
		}
		resp.Result = &hostResult
		if !hostResult.Success {
			resp.Error = hostResult.Error
		}
	}

	if resp.Error == "" && resultMsg.Status != "SUCCESS" {
		resp.Error = fmt.Sprintf("task status: %s", resultMsg.Status)
	}

	return resp, nil
}
