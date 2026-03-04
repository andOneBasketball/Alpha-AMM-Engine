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
		gin_zap.Ginzap(logger.Log, time.RFC3339, false),
		gin_zap.RecoveryWithZap(logger.Log, true),
	)

	apiGroup := r.Group("api/v1")

	// 支付相关接口
	exampleGroup := apiGroup.Group("example")
	{
		exampleGroup.POST("hello", v1.Hello)
	}
}
