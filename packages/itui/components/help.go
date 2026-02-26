package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	helpKeyColWidth = 22 // wide enough for "Up / Down / j / k" + padding
	helpModalOuter  = 72 // outer modal width
)

var (
	helpTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EAB308")).
			Bold(true)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB"))

	helpSectionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F59E0B")).
				Bold(true)

	helpDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))
)

type HelpModel struct {
	Visible      bool
	viewport     viewport.Model
	windowWidth  int
	windowHeight int
}

func NewHelp() HelpModel {
	vp := viewport.New(helpModalOuter-8, 24)
	vp.SetContent(helpContent(helpModalOuter - 8))
	return HelpModel{viewport: vp}
}

func (m *HelpModel) Show() {
	m.Visible = true
	m.viewport.GotoTop()
}

func (m *HelpModel) Hide() {
	m.Visible = false
}

// SetSize adapts the modal to the current terminal dimensions.
func (m *HelpModel) SetSize(w, h int) {
	m.windowWidth = w
	m.windowHeight = h

	outerWidth := helpModalOuter
	if w-4 < outerWidth {
		outerWidth = w - 4
	}
	// inner = outer - 2 borders - 2*2 padding
	innerWidth := outerWidth - 6
	if innerWidth < 20 {
		innerWidth = 20
	}

	// viewport height = window height minus modal chrome:
	// 2 border + 2 top/bottom padding + 1 title + 1 blank + 2 close hint lines = ~8
	vpHeight := h - 10
	if vpHeight < 5 {
		vpHeight = 5
	}

	m.viewport.Width = innerWidth
	m.viewport.Height = vpHeight
	m.viewport.SetContent(helpContent(innerWidth))
}

func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	if !m.Visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "?", "q"))):
			m.Visible = false
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m HelpModel) View() string {
	if !m.Visible {
		return ""
	}

	outerWidth := helpModalOuter
	if m.windowWidth > 0 && m.windowWidth-4 < outerWidth {
		outerWidth = m.windowWidth - 4
	}

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#EAB308")).
		Padding(1, 2).
		Width(outerWidth)

	scrollHint := ""
	if m.viewport.TotalLineCount() > m.viewport.Height {
		pct := int(m.viewport.ScrollPercent() * 100)
		scrollHint = helpDimStyle.Render(fmt.Sprintf(" (%d%%)", pct))
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		helpTitleStyle.Render("ITUI Keyboard Shortcuts")+scrollHint,
		"",
		m.viewport.View(),
		"",
		helpDimStyle.Render("Press Esc or ? to close   ↑↓ to scroll"),
	)

	return modalStyle.Render(content)
}

func helpContent(innerWidth int) string {
	descWidth := innerWidth - helpKeyColWidth - 2
	if descWidth < 10 {
		descWidth = 10
	}

	sections := []struct {
		title string
		binds []struct{ key, desc string }
	}{
		{
			title: "Navigation",
			binds: []struct{ key, desc string }{
				{"Tab / Shift+Tab", "Switch between panes"},
				{"Up / Down / j / k", "Navigate secret list"},
				{"Enter", "Select / expand secret"},
				{"Ctrl+P", "Focus AI prompt bar"},
			},
		},
		{
			title: "Secrets",
			binds: []struct{ key, desc string }{
				{"r", "Reveal / mask secret value"},
				{"n", "Create new secret"},
				{"X", "Delete selected secret"},
				{"e", "Switch environment"},
				{"/", "Search / filter secrets"},
				{"R", "Refresh secrets"},
				{"d", "Compare selected secret across envs"},
				{"p", "View propagation across envs"},
			},
		},
		{
			title: "AI Prompt",
			binds: []struct{ key, desc string }{
				{"Ctrl+P", "Focus prompt bar"},
				{"Enter", "Send prompt / execute command"},
				{"Esc", "Cancel / clear prompt"},
			},
		},
		{
			title: "Clipboard & Tools",
			binds: []struct{ key, desc string }{
				{"Ctrl+K", "Command palette"},
				{"c", "Copy value / output"},
				{"Ctrl+L", "Copy CLI deep link"},
				{"Ctrl+V", "Paste & analyze output"},
			},
		},
		{
			title: "General",
			binds: []struct{ key, desc string }{
				{"?", "Toggle this help"},
				{"q / Ctrl+C", "Quit ITUI"},
			},
		},
	}

	var b strings.Builder
	for _, section := range sections {
		b.WriteString(helpSectionStyle.Render(section.title))
		b.WriteString("\n")
		for _, bind := range section.binds {
			key := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981")).
				Bold(true).
				Width(helpKeyColWidth).
				Render(bind.key)
			desc := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F9FAFB")).
				Width(descWidth).
				Render(bind.desc)
			b.WriteString(fmt.Sprintf("  %s  %s\n", key, desc))
		}
		b.WriteString("\n")
	}

	b.WriteString(helpSectionStyle.Render("Example AI Prompts"))
	b.WriteString("\n")
	examples := []string{
		"show me all production secrets",
		"set DATABASE_URL to postgres://... in staging",
		"delete the old API key in dev",
		"compare staging and prod secrets",
		"export all dev secrets as .env",
	}
	for _, ex := range examples {
		b.WriteString(fmt.Sprintf("  %s\n",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#FDE68A")).Italic(true).Render("\""+ex+"\"")))
	}

	return b.String()
}
