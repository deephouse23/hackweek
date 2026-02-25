package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

type DetailMode int

const (
	DetailModeSecret DetailMode = iota
	DetailModeOutput
	DetailModeWelcome
	DetailModeSecretList
	DetailModeDiff
	DetailModePropagation
)

var (
	detailBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#374151")).
			Padding(0, 1)

	detailActiveBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#EAB308")).
				Padding(0, 1)

	dLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Width(12)

	dValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB"))

	dKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)

	dMaskedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))

	dErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))

	dSuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981"))

	dChangedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))

	dTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EAB308")).
			Bold(true).
			Padding(0, 0, 1, 0)
)

// DiffEntry represents a secret that exists in both environments but with different values.
type DiffEntry struct {
	Key    string
	ValueA string
	ValueB string
}

// PropagationEntry represents a single environment's state for a given secret key.
type PropagationEntry struct {
	Env            string
	Exists         bool
	Value          string
	MatchesCurrent bool
}

type DetailPaneModel struct {
	viewport viewport.Model
	Active   bool
	Mode     DetailMode
	Width    int
	Height   int

	// Secret detail
	SecretKey     string
	SecretValue   string
	SecretType    string
	SecretPath    string
	SecretComment string
	ValueRevealed bool

	// Command output
	OutputTitle   string
	OutputContent string
	OutputIsError bool

	// Secret list (AI command result)
	SecretListTitle string
	SecretList      []SecretItem

	// Diff view
	DiffEnvA      string
	DiffEnvB      string
	DiffOnlyInA   []string
	DiffOnlyInB   []string
	DiffChanged   []DiffEntry
	DiffSameCount int

	// Propagation view
	PropagationKey        string
	PropagationCurrentEnv string
	PropagationEntries    []PropagationEntry
}

func NewDetailPane() DetailPaneModel {
	vp := viewport.New(30, 10)
	return DetailPaneModel{
		viewport: vp,
		Mode:     DetailModeWelcome,
	}
}

func (m *DetailPaneModel) SetSize(width, height int) {
	m.Width = width
	m.Height = height
	m.viewport.Width = width - 4 // border + padding
	m.viewport.Height = height - 4
}

func (m *DetailPaneModel) SetSecret(key, value, secretType, path, comment string) {
	m.Mode = DetailModeSecret
	m.SecretKey = key
	m.SecretValue = value
	m.SecretType = secretType
	m.SecretPath = path
	m.SecretComment = comment
	m.ValueRevealed = false
	m.updateViewportContent()
}

func (m *DetailPaneModel) SetOutput(title, content string, isError bool) {
	m.Mode = DetailModeOutput
	m.OutputTitle = title
	m.OutputContent = content
	m.OutputIsError = isError
	m.updateViewportContent()
}

// SetSecretList displays a formatted list of secrets with masked values.
func (m *DetailPaneModel) SetSecretList(title string, secrets []SecretItem) {
	m.Mode = DetailModeSecretList
	m.SecretListTitle = title
	m.SecretList = secrets
	m.ValueRevealed = false
	m.updateViewportContent()
}

// ResetToWelcome returns the detail pane to the welcome/home screen.
func (m *DetailPaneModel) ResetToWelcome() {
	m.Mode = DetailModeWelcome
	m.ValueRevealed = false
	m.updateViewportContent()
}

// SetDiff displays a color-coded diff between two environments.
func (m *DetailPaneModel) SetDiff(envA, envB string, onlyInA, onlyInB []string, changed []DiffEntry, sameCount int) {
	m.Mode = DetailModeDiff
	m.DiffEnvA = envA
	m.DiffEnvB = envB
	m.DiffOnlyInA = onlyInA
	m.DiffOnlyInB = onlyInB
	m.DiffChanged = changed
	m.DiffSameCount = sameCount
	m.ValueRevealed = false
	m.updateViewportContent()
}

// SetPropagation displays a secret's presence across all environments.
func (m *DetailPaneModel) SetPropagation(key string, currentEnv string, entries []PropagationEntry) {
	m.Mode = DetailModePropagation
	m.PropagationKey = key
	m.PropagationCurrentEnv = currentEnv
	m.PropagationEntries = entries
	m.ValueRevealed = false
	m.updateViewportContent()
}

// CopyableContent returns the most relevant text for clipboard copy.
// For secrets: the value. For command output: the output content.
func (m *DetailPaneModel) CopyableContent() string {
	switch m.Mode {
	case DetailModeSecret:
		return m.SecretValue
	case DetailModeOutput:
		return m.OutputContent
	case DetailModeSecretList:
		var b strings.Builder
		for _, s := range m.SecretList {
			b.WriteString(fmt.Sprintf("%s=%s\n", s.KeyName, s.Value))
		}
		return b.String()
	case DetailModeDiff:
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Diff: %s vs %s\n", m.DiffEnvA, m.DiffEnvB))
		for _, k := range m.DiffOnlyInA {
			b.WriteString(fmt.Sprintf("+ %s (only in %s)\n", k, m.DiffEnvA))
		}
		for _, k := range m.DiffOnlyInB {
			b.WriteString(fmt.Sprintf("- %s (only in %s)\n", k, m.DiffEnvB))
		}
		for _, d := range m.DiffChanged {
			b.WriteString(fmt.Sprintf("~ %s\n", d.Key))
		}
		return b.String()
	case DetailModePropagation:
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Propagation: %s\n", m.PropagationKey))
		for _, e := range m.PropagationEntries {
			if !e.Exists {
				b.WriteString(fmt.Sprintf("  %s: missing\n", e.Env))
			} else if e.MatchesCurrent {
				b.WriteString(fmt.Sprintf("  %s: matches\n", e.Env))
			} else {
				b.WriteString(fmt.Sprintf("  %s: different\n", e.Env))
			}
		}
		return b.String()
	default:
		return ""
	}
}

func (m *DetailPaneModel) ToggleReveal() {
	if m.Mode == DetailModeSecret || m.Mode == DetailModeSecretList || m.Mode == DetailModeDiff || m.Mode == DetailModePropagation {
		m.ValueRevealed = !m.ValueRevealed
		m.updateViewportContent()
	}
}

func (m *DetailPaneModel) updateViewportContent() {
	var content string

	switch m.Mode {
	case DetailModeSecret:
		content = m.renderSecretDetail()
	case DetailModeOutput:
		content = m.renderOutput()
	case DetailModeWelcome:
		content = m.renderWelcome()
	case DetailModeSecretList:
		content = m.renderSecretList()
	case DetailModeDiff:
		content = m.renderDiff()
	case DetailModePropagation:
		content = m.renderPropagation()
	}

	m.viewport.SetContent(content)
}

// wrapText wraps text to fit within the viewport width.
func (m *DetailPaneModel) wrapText(s string) string {
	w := m.viewport.Width
	if w <= 0 {
		return s
	}
	return wordwrap.String(s, w)
}

func (m *DetailPaneModel) renderSecretDetail() string {
	var b strings.Builder

	b.WriteString(dTitleStyle.Render("Secret Detail"))
	b.WriteString("\n\n")

	b.WriteString(dLabelStyle.Render("Key:"))
	b.WriteString("  ")
	b.WriteString(dKeyStyle.Render(m.SecretKey))
	b.WriteString("\n\n")

	b.WriteString(dLabelStyle.Render("Value:"))
	b.WriteString("  ")
	if m.ValueRevealed {
		b.WriteString(dValueStyle.Render(m.wrapText(m.SecretValue)))
	} else {
		b.WriteString(dMaskedStyle.Render("••••••••  [press r to reveal]"))
	}
	b.WriteString("\n\n")

	b.WriteString(dLabelStyle.Render("Type:"))
	b.WriteString("  ")
	b.WriteString(dValueStyle.Render(m.SecretType))
	b.WriteString("\n\n")

	b.WriteString(dLabelStyle.Render("Path:"))
	b.WriteString("  ")
	b.WriteString(dValueStyle.Render(m.SecretPath))

	if m.SecretComment != "" {
		b.WriteString("\n\n")
		b.WriteString(dLabelStyle.Render("Comment:"))
		b.WriteString("  ")
		b.WriteString(dValueStyle.Render(m.wrapText(m.SecretComment)))
	}

	return b.String()
}

func (m *DetailPaneModel) renderSecretList() string {
	var b strings.Builder

	// Title with count
	title := fmt.Sprintf("%s — %d secret", m.SecretListTitle, len(m.SecretList))
	if len(m.SecretList) != 1 {
		title += "s"
	}
	b.WriteString(dTitleStyle.Render(title))
	b.WriteString("\n")

	// Reveal hint
	if m.ValueRevealed {
		b.WriteString(dMaskedStyle.Render("  [press r to hide]"))
	} else {
		b.WriteString(dMaskedStyle.Render("  [press r to reveal]"))
	}
	b.WriteString("\n\n")

	// Find max key length for alignment
	maxKeyLen := 0
	for _, s := range m.SecretList {
		if len(s.KeyName) > maxKeyLen {
			maxKeyLen = len(s.KeyName)
		}
	}
	if maxKeyLen > 30 {
		maxKeyLen = 30
	}

	// Render each secret as a row
	for _, s := range m.SecretList {
		keyPadded := s.KeyName
		if len(keyPadded) < maxKeyLen {
			keyPadded += strings.Repeat(" ", maxKeyLen-len(keyPadded))
		}

		b.WriteString("  ")
		b.WriteString(dKeyStyle.Render(keyPadded))
		b.WriteString("  ")

		if m.ValueRevealed {
			// Wrap value accounting for indentation and key column width
			valueWidth := m.viewport.Width - maxKeyLen - 6 // 2 indent + 2 key padding + 2 value padding
			if valueWidth > 20 {
				b.WriteString(dValueStyle.Render(wordwrap.String(s.Value, valueWidth)))
			} else {
				b.WriteString(dValueStyle.Render(s.Value))
			}
		} else {
			b.WriteString(dMaskedStyle.Render("••••••••"))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m *DetailPaneModel) renderOutput() string {
	var b strings.Builder

	title := dTitleStyle.Render(m.OutputTitle)
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.OutputIsError {
		b.WriteString(dErrorStyle.Render(m.wrapText(m.OutputContent)))
	} else {
		b.WriteString(dValueStyle.Render(m.wrapText(m.OutputContent)))
	}

	return b.String()
}

func (m *DetailPaneModel) renderDiff() string {
	var b strings.Builder

	title := fmt.Sprintf("Diff: %s vs %s", m.DiffEnvA, m.DiffEnvB)
	b.WriteString(dTitleStyle.Render(title))
	b.WriteString("\n")

	if m.ValueRevealed {
		b.WriteString(dMaskedStyle.Render("  [press r to hide values]"))
	} else {
		b.WriteString(dMaskedStyle.Render("  [press r to reveal values]"))
	}
	b.WriteString("\n\n")

	total := len(m.DiffOnlyInA) + len(m.DiffOnlyInB) + len(m.DiffChanged)
	if total == 0 {
		b.WriteString(dSuccessStyle.Render("Environments are identical"))
		b.WriteString(fmt.Sprintf(" (%d secrets)\n", m.DiffSameCount))
		return b.String()
	}

	summaryLine := fmt.Sprintf("%d only in %s, %d only in %s, %d different, %d identical",
		len(m.DiffOnlyInA), m.DiffEnvA,
		len(m.DiffOnlyInB), m.DiffEnvB,
		len(m.DiffChanged), m.DiffSameCount)
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(summaryLine))
	b.WriteString("\n\n")

	if len(m.DiffOnlyInA) > 0 {
		header := fmt.Sprintf("Only in %s (%d):", m.DiffEnvA, len(m.DiffOnlyInA))
		b.WriteString(dSuccessStyle.Render(header))
		b.WriteString("\n")
		for _, key := range m.DiffOnlyInA {
			b.WriteString(dSuccessStyle.Render("  + " + key))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(m.DiffOnlyInB) > 0 {
		header := fmt.Sprintf("Only in %s (%d):", m.DiffEnvB, len(m.DiffOnlyInB))
		b.WriteString(dErrorStyle.Render(header))
		b.WriteString("\n")
		for _, key := range m.DiffOnlyInB {
			b.WriteString(dErrorStyle.Render("  - " + key))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(m.DiffChanged) > 0 {
		header := fmt.Sprintf("Different values (%d):", len(m.DiffChanged))
		b.WriteString(dChangedStyle.Render(header))
		b.WriteString("\n")
		for _, d := range m.DiffChanged {
			b.WriteString(dChangedStyle.Render("  ~ " + d.Key))
			b.WriteString("\n")
			if m.ValueRevealed {
				b.WriteString(fmt.Sprintf("    %s: %s\n", m.DiffEnvA, dValueStyle.Render(m.wrapText(d.ValueA))))
				b.WriteString(fmt.Sprintf("    %s: %s\n", m.DiffEnvB, dValueStyle.Render(m.wrapText(d.ValueB))))
			} else {
				b.WriteString(fmt.Sprintf("    %s: %s\n", m.DiffEnvA, dMaskedStyle.Render("••••••••")))
				b.WriteString(fmt.Sprintf("    %s: %s\n", m.DiffEnvB, dMaskedStyle.Render("••••••••")))
			}
		}
	}

	return b.String()
}

func (m *DetailPaneModel) renderPropagation() string {
	var b strings.Builder

	b.WriteString(dTitleStyle.Render("Secret Propagation: " + m.PropagationKey))
	b.WriteString("\n")

	if m.ValueRevealed {
		b.WriteString(dMaskedStyle.Render("  [press r to hide values]"))
	} else {
		b.WriteString(dMaskedStyle.Render("  [press r to reveal values]"))
	}
	b.WriteString("\n\n")

	// Find max env name length for alignment
	maxEnvLen := 0
	for _, e := range m.PropagationEntries {
		if len(e.Env) > maxEnvLen {
			maxEnvLen = len(e.Env)
		}
	}

	for _, entry := range m.PropagationEntries {
		envPadded := entry.Env
		if len(envPadded) < maxEnvLen {
			envPadded += strings.Repeat(" ", maxEnvLen-len(envPadded))
		}

		marker := "  "
		if entry.Env == m.PropagationCurrentEnv {
			marker = "* "
		}

		var statusIcon, valueDisplay string
		if !entry.Exists {
			statusIcon = dErrorStyle.Render("X")
			valueDisplay = dErrorStyle.Render("(not set)")
		} else if entry.MatchesCurrent {
			statusIcon = dSuccessStyle.Render("=")
			if m.ValueRevealed {
				valueDisplay = dValueStyle.Render(m.wrapText(entry.Value))
			} else {
				valueDisplay = dMaskedStyle.Render("••••••••")
			}
		} else {
			statusIcon = dChangedStyle.Render("~")
			if m.ValueRevealed {
				valueDisplay = dChangedStyle.Render(m.wrapText(entry.Value))
			} else {
				valueDisplay = dMaskedStyle.Render("••••••••")
			}
		}

		b.WriteString(fmt.Sprintf("%s%s  %s  %s\n",
			marker,
			dKeyStyle.Render(envPadded),
			statusIcon,
			valueDisplay,
		))
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(
		"Legend: "))
	b.WriteString(dSuccessStyle.Render("= matches") + "  ")
	b.WriteString(dChangedStyle.Render("~ different") + "  ")
	b.WriteString(dErrorStyle.Render("X missing"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(
		"* = current environment"))

	return b.String()
}

func (m *DetailPaneModel) renderWelcome() string {
	var b strings.Builder

	b.WriteString(dTitleStyle.Render("Welcome to Infisical TUI"))
	b.WriteString("\n\n")
	b.WriteString(dLabelStyle.Render("Get started:"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s  %s\n", dKeyStyle.Render("Ctrl+P"), "Focus AI prompt"))
	b.WriteString(fmt.Sprintf("  %s  %s\n", dKeyStyle.Render("Enter"), "View secret detail"))
	b.WriteString(fmt.Sprintf("  %s  %s\n", dKeyStyle.Render("e"), "Switch environment"))
	b.WriteString(fmt.Sprintf("  %s  %s\n", dKeyStyle.Render("n"), "Create new secret"))
	b.WriteString(fmt.Sprintf("  %s  %s\n", dKeyStyle.Render("?"), "Show all shortcuts"))

	return b.String()
}

func (m DetailPaneModel) Update(msg tea.Msg) (DetailPaneModel, tea.Cmd) {
	if !m.Active {
		return m, nil
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m DetailPaneModel) View() string {
	style := detailBorder
	if m.Active {
		style = detailActiveBorder
	}

	if m.Width > 0 {
		style = style.Width(m.Width - 2)
	}
	if m.Height > 0 {
		style = style.Height(m.Height - 2)
	}

	m.updateViewportContent()
	return style.Render(m.viewport.View())
}
