// Package cli provides various parameter calling methods and descriptions.
package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "butler",
	Short: "A cyber butler which can scheduled notify",
}

func Execute() error {
	err := rootCmd.Execute()
	return err
}
