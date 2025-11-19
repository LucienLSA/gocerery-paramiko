// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"gocerery/internal/svc"
	"gocerery/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GocereryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGocereryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GocereryLogic {
	return &GocereryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GocereryLogic) Gocerery(req *types.Request) (resp *types.Response, err error) {
	// todo: add your logic here and delete this line

	return
}
