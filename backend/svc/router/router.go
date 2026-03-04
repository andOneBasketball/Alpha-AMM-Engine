package router

import (
	"alpha-amm-engine/pkg/logger"
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RunWithContext starts the HTTP server and gracefully shuts it down on ctx.Done().
// We intentionally implement it in this package to avoid leaking HTTP details to main.
func RunWithContext(ctx context.Context, addr string, debug bool) {
	//设置启动模式
	switch debug {
	case true:
		gin.SetMode(gin.DebugMode)
	case false:
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	initWebRouter(r)

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		if err := r.Run(addr); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal("run web server error: ", zap.Any("err", err))
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("server shutdown error", zap.Any("err", err))
	}
}
