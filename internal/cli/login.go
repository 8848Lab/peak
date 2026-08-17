package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with your 8848 Lab account",
	RunE: func(cmd *cobra.Command, args []string) error {
		tag := lipgloss.NewStyle().Foreground(orange).Bold(true)
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b6b6b"))

		fmt.Printf("\n  %s\n\n", tag.Render("▲ peak login"))
		fmt.Println(dim.Render("  Opening Himalaya in your browser..."))
		fmt.Println(dim.Render("  Waiting for authentication..."))

		// TODO: implement OAuth / token flow against Himalaya API
		// 1. Open browser to himalaya.8848lab.org/cli-auth?code=XXX
		// 2. Poll for token
		// 3. Save token to ~/.peak/config.toml

		fmt.Printf("\n  %s\n\n",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#4caf50")).Bold(true).Render("✓ Logged in"),
		)

		return nil
	},
}
