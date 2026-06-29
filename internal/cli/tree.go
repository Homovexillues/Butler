package cli

import (
	"fmt"

	"butler/internal/config"

	"github.com/spf13/cobra"
)

var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "展示当前所有任务及触发时间",
	RunE: func(cmd *cobra.Command, args []string) error {
		plan, err := config.LoadPlan()
		if err != nil {
			return fmt.Errorf("fail to load plan:\n%w", err)
		}
		err = plan.PrintTree()
		return err
	},
}

func init() {
	rootCmd.AddCommand(treeCmd)
}
