package cmd

import (
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Start blockchain scanner to collect AMM pool events",
	Long: `Blockchain scanner monitors logs from Uniswap V2/V3 pools 
and synchronizes liquidity events to the database.`,
	Run: func(cmd *cobra.Command, args []string) {

	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
