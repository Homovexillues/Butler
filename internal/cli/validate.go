package cli

import (
	"log"

	"butler/internal/config"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "验证配置文件语法",
	Run: func(cmd *cobra.Command, args []string) {
		var errs []error
		_, err := config.LoadConfig()
		if err != nil {
			log.Fatalf("fail to load config:\n%s", err.Error())
		}
		errs = config.ValidateConfig()
		plan, err := config.LoadPlan()
		if err != nil {
			log.Fatalf("fail to load plan:\n%s", err.Error())
		}
		errs = append(errs, plan.ValidatePlan()...)
		for _, err := range errs {
			log.Printf("%s\n", err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
