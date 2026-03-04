package v1

import (
	"alpha-amm-engine/pkg/logger"
	"alpha-amm-engine/pkg/models"
	"alpha-amm-engine/svc/service"

	"github.com/andOneBasketball/baseapi-go/pkg/web/xlhttp"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// example
func Hello(c *gin.Context) {
	var (
		err error
	)
	r := xlhttp.Build(c)

	var req models.HelloReq
	err = r.RequestParser(&req)
	if err != nil {
		return
	}
	req.ClientIP = c.ClientIP()

	resp, err := service.Hello(c, &req)
	if err != nil {
		logger.Log.Error("Hello error", zap.Any("err", err))
		r.JsonReturn(err)
		return
	}
	r.JsonReturn(err, resp)
}
