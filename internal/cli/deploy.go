package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/8848lab/peak/internal/api"
	"github.com/8848lab/peak/internal/archive"
	"github.com/8848lab/peak/pkg/config"
)

// ── Styles ────────────────────────────────────────────────────────────────

var (
	orange    = lipgloss.Color("#E8572A")
	darkBg    = lipgloss.Color("#1a1a1a")
	mutedText = lipgloss.Color("#6b6b6b")
	white     = lipgloss.Color("#f0f0f0")
	green     = lipgloss.Color("#4caf50")

	stepDone    = lipgloss.NewStyle().Foreground(orange).SetString("✓")
	stepPending = lipgloss.NewStyle().Foreground(mutedText)
	stepActive  = lipgloss.NewStyle().Foreground(white)

	envBadge = lipgloss.NewStyle().
			Background(orange).
			Foreground(white).
			Padding(0, 1).
			Bold(true)

	successBox = lipgloss.NewStyle().
			Background(lipgloss.Color("#2a1a0e")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(orange).
			Padding(0, 1)

	promptStyle = lipgloss.NewStyle().Foreground(orange).Bold(true)
)

// ── Steps ─────────────────────────────────────────────────────────────────

type step struct {
	label string
	done  bool
}

var deploySteps = []step{
	{label: "Archiving project"},
	{label: "Uploading"},
	{label: "Building"},
	{label: "Deploying"},
}

// ── Model ─────────────────────────────────────────────────────────────────

type deployModel struct {
	steps       []step
	current     int
	spinner     spinner.Model
	done        bool
	failed      bool
	errMsg      string
	projectURL  string
	elapsed     time.Duration
	startTime   time.Time
	environment string

	client       *api.Client
	projectID    string
	archivePath  string
	deploymentID string
}

type deploymentCreatedMsg struct{ id string }
type stepAdvanceMsg struct{}
type deployFailedMsg struct{ err error }
type deployReadyMsg struct{ url string }

func initialDeployModel(client *api.Client, projectID, archivePath, env string) deployModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(orange)

	return deployModel{
		steps:       deploySteps,
		spinner:     s,
		environment: env,
		startTime:   time.Now(),
		client:      client,
		projectID:   projectID,
		archivePath: archivePath,
	}
}

func (m deployModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, uploadArchiveCmd(m.client, m.projectID, m.archivePath))
}

func uploadArchiveCmd(client *api.Client, projectID, archivePath string) tea.Cmd {
	return func() tea.Msg {
		f, err := os.Open(archivePath)
		if err != nil {
			return deployFailedMsg{err}
		}
		defer f.Close()

		deployment, err := client.DeployLocal(projectID, f)
		if err != nil {
			return deployFailedMsg{err}
		}
		return deploymentCreatedMsg{deployment.ID}
	}
}

func pollDeploymentCmd(client *api.Client, deploymentID string) tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		deployment, err := client.GetDeployment(deploymentID)
		if err != nil {
			return deployFailedMsg{err}
		}
		switch deployment.Status {
		case "ready":
			url := ""
			if deployment.DeploymentURL != nil {
				url = *deployment.DeploymentURL
			}
			return deployReadyMsg{url}
		case "failed":
			logs := ""
			if deployment.Logs != nil {
				logs = *deployment.Logs
			}
			return deployFailedMsg{fmt.Errorf("build failed:\n%s", lastLines(logs, 15))}
		default:
			return stepAdvanceMsg{}
		}
	})
}

func lastLines(s string, n int) string {
	trimmed := strings.TrimRight(s, "\n")
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) <= n {
		return trimmed
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func (m deployModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case deploymentCreatedMsg:
		m.deploymentID = msg.id
		m.steps[0].done = true
		m.steps[1].done = true
		m.current = 2 // "Building"
		return m, pollDeploymentCmd(m.client, m.deploymentID)

	case stepAdvanceMsg:
		// Still building — the backend doesn't expose discrete build phases,
		// so this just keeps the spinner alive on "Building" while polling.
		return m, pollDeploymentCmd(m.client, m.deploymentID)

	case deployReadyMsg:
		for i := range m.steps {
			m.steps[i].done = true
		}
		m.current = len(m.steps)
		m.elapsed = time.Since(m.startTime)
		m.done = true
		m.projectURL = msg.url
		return m, tea.Quit

	case deployFailedMsg:
		m.failed = true
		m.errMsg = msg.err.Error()
		m.elapsed = time.Since(m.startTime)
		return m, tea.Quit
	}

	return m, nil
}

func (m deployModel) View() string {
	header := fmt.Sprintf(
		"%s peak deploy          %s\n\n",
		promptStyle.Render("$"),
		envBadge.Render("● "+m.environment),
	)

	var steps string
	for i, s := range m.steps {
		switch {
		case s.done:
			steps += fmt.Sprintf("  %s  %s\n", stepDone.Render(), lipgloss.NewStyle().Foreground(mutedText).Render(s.label))
		case i == m.current && !m.done && !m.failed:
			steps += fmt.Sprintf("  %s  %s\n", m.spinner.View(), stepActive.Render(s.label))
		default:
			steps += fmt.Sprintf("     %s\n", stepPending.Render(s.label))
		}
	}

	view := header + steps

	if m.done {
		result := successBox.Render(
			fmt.Sprintf("✓  Deployment ready\n   %s     %ds",
				lipgloss.NewStyle().Foreground(mutedText).Render(m.projectURL),
				int(m.elapsed.Seconds()),
			),
		)
		view += "\n" + result + "\n"
	}

	if m.failed {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e05252"))
		view += "\n" + errStyle.Render("✗  "+m.errMsg) + "\n"
	}

	return view
}

// ── Command ───────────────────────────────────────────────────────────────

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy your project to Himalaya",
	RunE: func(cmd *cobra.Command, args []string) error {
		env, _ := cmd.Flags().GetString("env")

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.Token == "" {
			return fmt.Errorf("not logged in — run `peak login` first")
		}
		client := api.NewClient(cfg.APIBaseURL, cfg.Token)

		wd, err := os.Getwd()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectLink(client, wd)
		if err != nil {
			return err
		}

		archivePath, err := buildArchive(wd)
		if err != nil {
			return fmt.Errorf("failed to archive project: %w", err)
		}
		defer os.Remove(archivePath)

		m := initialDeployModel(client, projectID, archivePath, env)
		p := tea.NewProgram(m)
		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("deploy failed: %w", err)
		}
		if fm, ok := finalModel.(deployModel); ok && fm.failed {
			return fmt.Errorf("deploy failed: %s", fm.errMsg)
		}
		return nil
	},
}

// resolveProjectLink returns the Himalaya project ID this directory deploys
// to — from .peak/project.json if it exists, or by prompting to create a
// new project (and writing the link file) if not.
func resolveProjectLink(client *api.Client, dir string) (string, error) {
	link, err := config.LoadProjectLink(dir)
	if err == nil {
		return link.ProjectID, nil
	}

	fmt.Println(lipgloss.NewStyle().Foreground(mutedText).Render("No linked project found in this directory — let's create one."))

	orgs, err := client.ListOrganizations()
	if err != nil {
		return "", fmt.Errorf("could not list organizations: %w", err)
	}
	if len(orgs) == 0 {
		return "", fmt.Errorf("no organizations found for your account")
	}

	reader := bufio.NewReader(os.Stdin)
	var org api.Organization
	if len(orgs) == 1 {
		org = orgs[0]
		fmt.Printf("Using organization: %s\n", org.Name)
	} else {
		fmt.Println("Select an organization:")
		for i, o := range orgs {
			fmt.Printf("  %d. %s\n", i+1, o.Name)
		}
		fmt.Print("> ")
		choice, _ := reader.ReadString('\n')
		idx, convErr := strconv.Atoi(strings.TrimSpace(choice))
		if convErr != nil || idx < 1 || idx > len(orgs) {
			return "", fmt.Errorf("invalid selection")
		}
		org = orgs[idx-1]
	}

	fmt.Print("Project name: ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(dir)
	}

	project, err := client.CreateProject(api.CreateProjectRequest{OrganizationID: org.ID, Name: name})
	if err != nil {
		return "", fmt.Errorf("could not create project: %w", err)
	}

	if err := config.SaveProjectLink(dir, &config.ProjectLink{
		ProjectID:      project.ID,
		OrganizationID: org.ID,
		Name:           project.Name,
	}); err != nil {
		return "", fmt.Errorf("could not save project link: %w", err)
	}
	if err := config.EnsureGitignored(dir); err != nil {
		fmt.Println(lipgloss.NewStyle().Foreground(mutedText).Render("warning: could not update .gitignore: " + err.Error()))
	}

	return project.ID, nil
}

func buildArchive(dir string) (string, error) {
	f, err := os.CreateTemp("", "peak-deploy-*.tar.gz")
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := archive.TarGz(dir, f); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func init() {
	deployCmd.Flags().StringP("env", "e", "production", "Target environment (production, staging, preview)")
}
