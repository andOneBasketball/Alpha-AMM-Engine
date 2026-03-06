package router

import (
	"alpha-amm-engine/pkg/logger"
	v1 "alpha-amm-engine/svc/api/v1"
	"time"

	"github.com/andOneBasketball/baseapi-go/pkg/web/gin_zap"

	"github.com/gin-gonic/gin"
)

func initWebRouter(r *gin.Engine) {
	r.Use(
		gin_zap.Ginzap(logger.Log.Logger, time.RFC3339, false),
		gin_zap.RecoveryWithZap(logger.Log.Logger, true),
	)

	apiGroup := r.Group("api/v1")

	// 支付相关接口
	ammGroup := apiGroup.Group("amm")
	{
		ammGroup.GET("slippage_curve", v1.SlippageCurve)
	}
}
