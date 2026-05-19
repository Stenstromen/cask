package cmd

import (
	"os"

	"github.com/stenstromen/cask/resource"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cask",
	Short: "Cloudflare ASK",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&resource.ConfigPathOverride, "config", "", "Path to config file (overrides CASKCONFIG and ~/.cask.yaml)")
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
