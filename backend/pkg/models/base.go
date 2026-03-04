package models

type BaseReq struct {
	ClientIP string // 客户端IP
}

type CommonResp struct {
	Data any `json:"data"`
}
