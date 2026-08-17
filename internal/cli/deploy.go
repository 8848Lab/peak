package cli

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	orange      = lipgloss.Color("#E8572A")
	darkBg      = lipgloss.Color("#1a1a1a")
	mutedText   = lipgloss.Color("#6b6b6b")
	white       = lipgloss.Color("#f0f0f0")
	green       = lipgloss.Color("#4caf50")

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

// ── Steps ─────────────────────────────────────────────────────────────────────

type step struct {
	label string
	done  bool
}

var deploySteps = []step{
	{label: "Cloning repository"},
	{label: "Installing dependencies"},
	{label: "Building application"},
	{label: "Optimizing assets"},
}

// ── Model ─────────────────────────────────────────────────────────────────────

type deployModel struct {
	steps       []step
	current     int
	spinner     spinner.Model
	done        bool
	projectURL  string
	elapsed     time.Duration
	startTime   time.Time
	environment string
}

type stepDoneMsg struct{}
type deployDoneMsg struct{ url string }

func initialDeployModel(env string) deployModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(orange)

	return deployModel{
		steps:       deploySteps,
		spinner:     s,
		environment: env,
		startTime:   time.Now(),
	}
}

func (m deployModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, tickStep())
}

func tickStep() tea.Cmd {
	return tea.Tick(800*time.Millisecond, func(t time.Time) tea.Msg {
		return stepDoneMsg{}
	})
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

	case stepDoneMsg:
		if m.current < len(m.steps) {
			m.steps[m.current].done = true
			m.current++
		}
		if m.current >= len(m.steps) {
			m.elapsed = time.Since(m.startTime)
			m.done = true
			m.projectURL = "your-project.8848.app"
			return m, tea.Quit
		}
		return m, tickStep()
	}

	return m, nil
}

func (m deployModel) View() string {
	header := fmt.Sprintf(
		"%s 8848 deploy          %s\n\n",
		promptStyle.Render("$"),
		envBadge.Render("● "+m.environment),
	)

	var steps string
	for i, s := range m.steps {
		switch {
		case s.done:
			steps += fmt.Sprintf("  %s  %s\n",
				stepDone.Render(),
				lipgloss.NewStyle().Foreground(mutedText).Render(s.label),
			)
		case i == m.current && !m.done:
			steps += fmt.Sprintf("  %s  %s\n",
				m.spinner.View(),
				stepActive.Render(s.label),
			)
		default:
			steps += fmt.Sprintf("     %s\n",
				stepPending.Render(s.label),
			)
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

	return view
}

// ── Command ───────────────────────────────────────────────────────────────────

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy your project to Himalaya",
	RunE: func(cmd *cobra.Command, args []string) error {
		env, _ := cmd.Flags().GetString("env")
		m := initialDeployModel(env)
		p := tea.NewProgram(m)
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("deploy failed: %w", err)
		}
		return nil
	},
}

func init() {
	deployCmd.Flags().StringP("env", "e", "production", "Target environment (production, staging, preview)")
}
