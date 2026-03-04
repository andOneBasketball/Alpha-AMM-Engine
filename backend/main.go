package main

import (
	"alpha-amm-engine/cmd"
	"alpha-amm-engine/pkg/config"
	"alpha-amm-engine/pkg/database"
	"alpha-amm-engine/pkg/logger"
	"context"
	"log"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

func main() {
	// 创建根 context，监听系统信号
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// 初始化日志
	cfg := config.Cfg
	logger.InitLogger(cfg.Log.Path)

	// 初始化数据库
	_, err = database.InitDatabase(&cfg.MySQL, cfg.Debug)
	if err != nil {
		logger.Log.Fatal("Failed to init database:", zap.Any("err", err))
	}
	logger.Log.Info("Load config success, Database init success", zap.Any("config", cfg))

	// 传递 context 给 cmd.Execute
	cmd.Execute(ctx)
}
