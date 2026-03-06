package v1

import (
	"alpha-amm-engine/pkg/logger"
	"alpha-amm-engine/pkg/models"
	"alpha-amm-engine/svc/service"
	"net/http"

	"github.com/andOneBasketball/baseapi-go/pkg/web/xlhttp"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SlippageCurve 计算滑点曲线
func SlippageCurve(c *gin.Context) {
	var (
		err error
	)
	r := xlhttp.Build(c)

	var req models.SlippageCurveReq
	err = r.RequestParser(&req)
	if err != nil {
		return
	}
	req.ClientIP = c.ClientIP()

	resp, err := service.SlippageCurve(c, &req)
	if err != nil {
		logger.Log.Error("SlippageCurve error", zap.Any("err", err))
		r.JsonReturn(err)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(resp.Data.(string)))
}
