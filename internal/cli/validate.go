package cli

import (
	"fmt"
	"log"

	"butler/internal/config"
	"butler/internal/parser"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "验证配置文件语法",
	Run: func(cmd *cobra.Command, args []string) {
		var errs []error
		cfg, err := config.LoadConfig()
		if err != nil {
			log.Fatalf("fail to load config:\n%s", err.Error())
		}
		errs = cfg.ValidateConfig()
		plan, err := config.LoadPlan()
		if err != nil {
			log.Fatalf("fail to load plan:\n%s", err.Error())
		}
		known := map[string]bool{}
		for _, channel := range parser.KnownChannels() {
			known[channel] = true
		}
		errs = append(errs, plan.ValidatePlan(known)...)
		for _, err := range errs {
			log.Fatalf("%s\n", err.Error())
		}
		fmt.Printf("Perfect!")
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
