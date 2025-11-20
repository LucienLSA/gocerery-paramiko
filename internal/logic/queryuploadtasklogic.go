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

type QueryUploadTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewQueryUploadTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryUploadTaskLogic {
	return &QueryUploadTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *QueryUploadTaskLogic) QueryUploadTask(req *types.SshTaskStatusRequest) (*types.UploadTaskStatusResponse, error) {
	if req.TaskID == "" {
		return nil, errors.New("task_id is required")
	}
	if l.svcCtx.CeleryBackend == nil {
		return nil, errors.New("celery backend is not configured")
	}

	resultMsg, err := l.svcCtx.CeleryBackend.GetResult(req.TaskID)
	if err != nil {
		l.Logger.Infof("upload task %s not finished yet: %v", req.TaskID, err)
		return &types.UploadTaskStatusResponse{
			TaskID: req.TaskID,
			Status: "PENDING",
			Error:  err.Error(),
		}, nil
	}

	resp := &types.UploadTaskStatusResponse{
		TaskID: req.TaskID,
		Status: resultMsg.Status,
	}

	if resultMsg.Result != nil {
		resultBytes, marshalErr := json.Marshal(resultMsg.Result)
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal task result: %w", marshalErr)
		}
		var rawResults []map[string]interface{}
		if err := json.Unmarshal(resultBytes, &rawResults); err != nil {
			return nil, fmt.Errorf("unmarshal upload task results: %w", err)
		}
		uploadResults := make([]types.UploadResult, 0, len(rawResults))
		for _, raw := range rawResults {
			ur := types.UploadResult{}
			if name, ok := raw["name"].(string); ok {
				ur.Name = name
			}
			if host, ok := raw["host"].(string); ok {
				ur.Host = host
			}
			if success, ok := raw["success"].(bool); ok {
				ur.Success = success
			}
			if errMsg, ok := raw["error"].(string); ok {
				ur.Error = errMsg
			}
			// 解析 uploaded_files
			if uploadedFiles, ok := raw["uploaded_files"].([]interface{}); ok {
				ur.UploadedFiles = make([]string, 0, len(uploadedFiles))
				for _, f := range uploadedFiles {
					if fileInfo, ok := f.(map[string]interface{}); ok {
						if remote, ok := fileInfo["remote"].(string); ok {
							ur.UploadedFiles = append(ur.UploadedFiles, remote)
						}
					}
				}
			}
			// 解析 failed_files
			if failedFiles, ok := raw["failed_files"].([]interface{}); ok {
				ur.FailedFiles = make([]string, 0, len(failedFiles))
				for _, f := range failedFiles {
					if fileInfo, ok := f.(map[string]interface{}); ok {
						if remote, ok := fileInfo["remote"].(string); ok {
							ur.FailedFiles = append(ur.FailedFiles, remote)
						}
					}
				}
			}
			uploadResults = append(uploadResults, ur)
		}
		resp.Results = uploadResults
		// Aggregate error from results if any target failed
		for _, ur := range uploadResults {
			if !ur.Success && ur.Error != "" {
				resp.Error = fmt.Sprintf("some targets failed: %s", ur.Error)
				break
			}
		}
	}

	if resp.Error == "" && resultMsg.Status != "SUCCESS" {
		resp.Error = fmt.Sprintf("task status: %s", resultMsg.Status)
	}

	return resp, nil
}
