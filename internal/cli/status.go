package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/8848lab/peak/internal/api"
	"github.com/8848lab/peak/pkg/config"
)

var statusCmd = &cobra.Command{
	Use:   "status [deployment-id]",
	Short: "Show deployment status and health",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var arg string
		if len(args) > 0 {
			arg = args[0]
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.Token == "" {
			return fmt.Errorf("not logged in — run `peak login` first")
		}
		client := api.NewClient(cfg.APIBaseURL, cfg.Token)

		deploymentID, err := resolveDeploymentID(client, arg)
		if err != nil {
			return err
		}

		deployment, err := client.GetDeployment(deploymentID)
		if err != nil {
			if aerr := authError(err); aerr != nil {
				return aerr
			}
			return fmt.Errorf("could not fetch status: %w", err)
		}

		label := lipgloss.NewStyle().Foreground(mutedText)
		value := lipgloss.NewStyle().Foreground(white).Bold(true)
		statusStyle := lipgloss.NewStyle().Bold(true)
		switch deployment.Status {
		case "ready":
			statusStyle = statusStyle.Foreground(green)
		case "failed":
			statusStyle = statusStyle.Foreground(lipgloss.Color("#e05252"))
		default:
			statusStyle = statusStyle.Foreground(orange)
		}

		url := "—"
		if deployment.DeploymentURL != nil {
			url = *deployment.DeploymentURL
		}

		fmt.Printf("\n  %s\n\n", lipgloss.NewStyle().Foreground(orange).Bold(true).Render("▲ peak status"))
		fmt.Printf("  %s  %s\n", label.Render("deployment"), value.Render(deployment.ID))
		fmt.Printf("  %s  %s\n", label.Render("status    "), statusStyle.Render("● "+deployment.Status))
		fmt.Printf("  %s  %s\n\n", label.Render("url       "), value.Render(url))

		return nil
	},
}
