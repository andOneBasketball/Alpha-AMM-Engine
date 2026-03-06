package models

import "github.com/shopspring/decimal"

type SlippageCurveReq struct {
	BaseReq
	Token0Addr    string          `form:"token0_addr"    binding:"required"`
	Token1Addr    string          `form:"token1_addr"    binding:"required"`
	SamplingSteps decimal.Decimal `form:"sampling_steps" binding:"required"` // 采样步长
	ChainId       string          `form:"chain_id"       binding:"required"`
}
