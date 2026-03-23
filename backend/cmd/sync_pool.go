package cmd

import (
	"alpha-amm-engine/pkg/config"
	"alpha-amm-engine/pkg/logger"
	"alpha-amm-engine/svc/poolsync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var syncMode string

var syncPoolCmd = &cobra.Command{
	Use:   "sync-pool",
	Short: "Synchronize AMM pool data to the database",
	Long: `Synchronizes the current state of AMM pools to the database.

Reads all token pairs from the config_pair table, fetches on-chain state
(reserves for V2, slot0/liquidity for V3), and upserts into alpha_amm_pool.

Modes:
  once   Run a single sync then exit (default)
  daily  Sync immediately, then repeat every day at 00:05 AM`,
	Run: func(cmd *cobra.Command, args []string) {
		chainConfig, ok := config.Cfg.Scan.Blockchain["1"]
		if !ok {
			logger.Log.Fatal("No configuration found for chain_id 1")
		}

		logger.Log.Info("Starting pool sync with config", zap.Any("chainConfig", chainConfig), zap.String("mode", syncMode))

		syncer, err := poolsync.NewPoolSyncer(chainConfig.RPC, 1, config.Cfg.SyncPool.RPCRatePerSecond)
		if err != nil {
			logger.Log.Fatal("Failed to create pool syncer", zap.Error(err))
		}
		defer syncer.Close()

		switch syncMode {
		case "once":
			syncer.SyncAll(rootCtx)
		case "daily":
			// 启动时立即同步一次
			syncer.SyncAll(rootCtx)

			c := cron.New(cron.WithLocation(time.Local))
			c.AddFunc("5 0 * * *", func() {
				syncer.SyncAll(rootCtx)
			})
			c.Start()

			<-rootCtx.Done()
			c.Stop()
		default:
			logger.Log.Fatal("Invalid --mode value, must be 'once' or 'daily'",
				zap.String("mode", syncMode))
		}
	},
}

func init() {
	syncPoolCmd.Flags().StringVar(&syncMode, "mode", "once", "sync mode: once (run once and exit) or daily (sync at 00:05 AM every day)")
	rootCmd.AddCommand(syncPoolCmd)
}
