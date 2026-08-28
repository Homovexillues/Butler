package cli

import (
	"fmt"

	"butler/internal/config"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "验证配置文件语法",
	RunE: func(cmd *cobra.Command, args []string) error {
		var errs []error
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("fail to load config:\n%w", err)
		}
		errs = cfg.ValidateConfig()
		plan, err := config.LoadPlan()
		if err != nil {
			return fmt.Errorf("fail to load plan:\n%w", err)
		}
		errs = append(errs, plan.ValidatePlan()...)
		for _, err := range errs {
			return fmt.Errorf("%w\n", err)
		}
		fmt.Printf("Perfect!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
