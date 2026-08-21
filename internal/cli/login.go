package cli

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cli/browser"
	"github.com/spf13/cobra"

	"github.com/8848lab/peak/internal/api"
	"github.com/8848lab/peak/pkg/config"
)

type loginModel struct {
	client   *api.Client
	start    *api.DeviceStartResponse
	deadline time.Time
	spinner  spinner.Model
	status   string // "starting" | "waiting" | "approved" | "error"
	err      error
}

type deviceStartedMsg struct{ start *api.DeviceStartResponse }
type deviceApprovedMsg struct{ resp *api.DevicePollResponse }
type deviceStillPendingMsg struct{}
type loginErrorMsg struct{ err error }

func initialLoginModel(client *api.Client) loginModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(orange)
	return loginModel{client: client, spinner: s, status: "starting"}
}

func startDeviceAuthCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		start, err := client.StartDeviceAuth()
		if err != nil {
			return loginErrorMsg{err}
		}
		return deviceStartedMsg{start}
	}
}

func pollOnceCmd(client *api.Client, deviceCode string, interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		resp, err := client.PollDeviceAuth(deviceCode)
		if err != nil {
			return loginErrorMsg{err}
		}
		if resp.Status == "approved" {
			return deviceApprovedMsg{resp}
		}
		return deviceStillPendingMsg{}
	})
}

func (m loginModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, startDeviceAuthCmd(m.client))
}

func (m loginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case deviceStartedMsg:
		m.start = msg.start
		m.status = "waiting"
		m.deadline = time.Now().Add(time.Duration(msg.start.ExpiresIn) * time.Second)
		verifyURL := fmt.Sprintf("%s?user_code=%s", msg.start.VerificationURI, msg.start.UserCode)
		_ = browser.OpenURL(verifyURL)
		interval := time.Duration(msg.start.Interval) * time.Second
		return m, pollOnceCmd(m.client, msg.start.DeviceCode, interval)

	case deviceStillPendingMsg:
		if time.Now().After(m.deadline) {
			m.status = "error"
			m.err = fmt.Errorf("login timed out — run `peak login` again")
			return m, tea.Quit
		}
		interval := time.Duration(m.start.Interval) * time.Second
		return m, pollOnceCmd(m.client, m.start.DeviceCode, interval)

	case deviceApprovedMsg:
		m.status = "approved"
		if err := config.SaveToken(msg.resp.Token); err != nil {
			m.status = "error"
			m.err = err
		}
		return m, tea.Quit

	case loginErrorMsg:
		m.status = "error"
		m.err = msg.err
		return m, tea.Quit
	}

	return m, nil
}

func (m loginModel) View() string {
	tag := lipgloss.NewStyle().Foreground(orange).Bold(true)
	dim := lipgloss.NewStyle().Foreground(mutedText)
	ok := lipgloss.NewStyle().Foreground(green).Bold(true)
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e05252")).Bold(true)

	switch m.status {
	case "starting":
		return fmt.Sprintf("\n  %s\n\n  %s %s\n\n", tag.Render("⛰ 8848 peak login"), m.spinner.View(), dim.Render("Starting..."))
	case "waiting":
		url := fmt.Sprintf("%s?user_code=%s", m.start.VerificationURI, m.start.UserCode)
		return fmt.Sprintf(
			"\n  %s\n\n  %s\n  %s\n\n  %s %s\n\n",
			tag.Render("⛰ 8848 peak login"),
			dim.Render("Opening your browser to confirm:"),
			lipgloss.NewStyle().Bold(true).Render(url),
			m.spinner.View(),
			dim.Render("Waiting for confirmation..."),
		)
	case "approved":
		return fmt.Sprintf("\n  %s\n\n", ok.Render("✓ Logged in"))
	case "error":
		return fmt.Sprintf("\n  %s  %s\n\n", errStyle.Render("✗ Login failed:"), m.err.Error())
	}
	return ""
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with your 8848 Lab account",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		client := api.NewClient(cfg.APIBaseURL, "")
		m := initialLoginModel(client)
		p := tea.NewProgram(m)
		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
		if lm, ok := finalModel.(loginModel); ok && lm.status == "error" {
			return fmt.Errorf("login failed: %w", lm.err)
		}
		return nil
	},
}
