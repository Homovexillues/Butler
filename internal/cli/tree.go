package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "展示当前所有任务及触发时间",
	RunE: func(cmd *cobra.Command, args []string) error {
		// todo: 待重构此查询命令
		return fmt.Errorf("fail to load plan:\n%w")
	},
}

func init() {
	rootCmd.AddCommand(treeCmd)
}
