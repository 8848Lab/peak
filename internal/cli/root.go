package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "peak",
	Short: "Peak — deploy and manage your infra via 8848 Lab",
	Long: `
  ▲ peak — by 8848 Lab

  The CLI for Himalaya. Deploy apps, tail logs, 
  manage environments — from your terminal.
`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(loginCmd)
}
