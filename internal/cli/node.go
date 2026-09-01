package cli

import (
	"github.com/spf13/cobra"
)

var nodeCmd = &cobra.Command{
	Use:   "node <crud>",
	Short: "对调度的节点进行增删改查",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		action := args[0]
		switch action {
		// todo: 待补充对调度节点的crud
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(nodeCmd)
}
