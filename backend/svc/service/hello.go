package service

import (
	"alpha-amm-engine/pkg/models"
	"context"

	"github.com/andOneBasketball/baseapi-go/pkg/utils"
)

func Hello(ctx context.Context, req *models.HelloReq) (resp *models.CommonResp, err error) {
	resp = &models.CommonResp{
		Data: map[string]string{
			"msg":      "Hello, World!",
			"username": req.Username,
			"password": utils.MaskMiddle(req.Password),
			"from":     req.ClientIP,
		},
	}
	return resp, nil
}
