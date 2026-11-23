// Package cli provides various parameter calling methods and descriptions.
package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "butler",
	Short: "A cyber butler which can scheduled notify",
	RunE: func(cmd *cobra.Command, args []string) error {
		return svc.Run()
	},
}

func Execute() error {
	err := rootCmd.Execute()
	return err
}
