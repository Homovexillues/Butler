package cli

import (
	"errors"
	"fmt"

	"butler/internal/config"
	"butler/internal/model"
	"butler/internal/parser"
	"butler/internal/store"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "验证配置文件及数据库中的所有任务",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var errs []error
		cfg, err := config.LoadConfig()
		if err != nil {
			errs = append(errs, fmt.Errorf("load config: %w", err))
		} else {
			errs = append(errs, cfg.ValidateConfig()...)
		}
		s, err := config.LoadDatabase()
		if err != nil {
			errs = append(errs, fmt.Errorf("load database: %w", err))
		} else {
			tasks, err := store.GetAll[model.Task](cmd.Context(), s)
			if err != nil {
				errs = append(errs, fmt.Errorf("load tasks: %w", err))
			} else {
				errs = append(errs, validateTasks(tasks))
			}
			if err := s.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close database: %w", err))
			}
		}
		if err := errors.Join(errs...); err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "配置及任务校验通过。")
		return err
	},
}

// Check disabled tasks too, without executing actions or sending notifications.
func validateTasks(tasks []model.Task) error {
	var errs []error
	for _, task := range tasks {
		err := parser.ValidateTask(task)
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func init() { rootCmd.AddCommand(validateCmd) }
