package cmd

import (
	"context"

	"alpha-amm-engine/pkg/config"
	"alpha-amm-engine/pkg/logger"
	"alpha-amm-engine/svc/scan"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Start blockchain scanner to collect AMM pool events",
	Long: `Blockchain scanner monitors logs from Uniswap V2/V3 pools
and synchronizes liquidity events to the database.`,
	Run: func(cmd *cobra.Command, args []string) {
		chainConfig, ok := config.Cfg.Scan.Blockchain["1"]
		if !ok {
			logger.Log.Fatal("No configuration found for chain_id 1")
		}

		runner, err := scan.NewRunner(&chainConfig)
		if err != nil {
			logger.Log.Fatal("Failed to create scanner runner", zap.Error(err))
		}

		ctx, cancel := context.WithCancel(rootCtx)
		defer cancel()

		runner.Start(ctx)

		// 阻塞等待，实际应用可能需要处理信号优雅退出
		select {
		case <-ctx.Done():
			runner.Stop()
		}
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
