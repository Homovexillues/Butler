// Package cli provides various parameter calling methods and descriptions.
package cli

import (
	"log"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "butler",
	Short: "A cyber butler which can scheduled notify",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Fail to execute butler command:%s", err.Error())
	}
}
