package models

import "github.com/shopspring/decimal"

type SlippageCurveReq struct {
	BaseReq
	Token0Addr    string          `form:"token0_addr"    binding:"required"`
	Token1Addr    string          `form:"token1_addr"    binding:"required"`
	SamplingStep0 decimal.Decimal `form:"sampling_step0" binding:"required"` // 采样步长
	SamplingStep1 decimal.Decimal `form:"sampling_step1" binding:"required"` // 采样步长
	ChainId       string          `form:"chain_id"       binding:"required"`
	V3Fee         int64           `form:"v3_fee"` // 可选，V3 手续费 ppm（500/3000/10000），不传则只展示 V2 曲线
}
