package cli

import (
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "前台运行调度器",
	RunE: func(cmd *cobra.Command, args []string) error {
		return svc.Run()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
