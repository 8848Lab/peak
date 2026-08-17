package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [project]",
	Short: "Show deployment status and health",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := "your-project"
		if len(args) > 0 {
			project = args[0]
		}

		label := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b6b6b"))
		value := lipgloss.NewStyle().Foreground(white).Bold(true)
		okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4caf50")).Bold(true)

		fmt.Printf("\n  %s\n\n", lipgloss.NewStyle().Foreground(orange).Bold(true).Render("▲ peak status"))
		fmt.Printf("  %s  %s\n", label.Render("project "), value.Render(project))
		fmt.Printf("  %s  %s\n", label.Render("status  "), okStyle.Render("● healthy"))
		fmt.Printf("  %s  %s\n", label.Render("url     "), value.Render(project+".8848.app"))
		fmt.Printf("  %s  %s\n", label.Render("region  "), value.Render("eu-west-1"))
		fmt.Printf("  %s  %s\n\n", label.Render("deploy  "), value.Render("just now"))

		// TODO: fetch real status from Himalaya API

		return nil
	},
}
