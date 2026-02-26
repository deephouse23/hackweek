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


// PropagationEntry represents a single environment's state for a given secret key.
type PropagationEntry struct {
	Env            string
	Exists         bool
	Value          string
	MatchesCurrent bool
}

// PropagationCopyRequestMsg is sent when the user wants to copy the current env's value to another env.
type PropagationCopyRequestMsg struct {
	TargetEnv string
}

// DiffCopyRequestMsg is sent when the user wants to copy a secret value from one env to the other in the diff view.
type DiffCopyRequestMsg struct {
	Key       string
	Value     string
	TargetEnv string
}

type DetailPaneModel struct {
	viewport  viewport.Model
	overlay   OverlayModel
	IsLoading bool
	Active    bool
	Mode      DetailMode
	Width     int
	Height    int

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
	DiffKey         string
	DiffEnvA        string
	DiffEnvB        string
	DiffValueA      string
	DiffValueB      string
	DiffMissingA    bool
	DiffMissingB    bool
	DiffSelectedCol int

	// Propagation view
	PropagationKey        string
	PropagationCurrentEnv string
	PropagationEntries    []PropagationEntry
	PropagationSelectedCol int
}

func NewDetailPane() DetailPaneModel {
	vp := viewport.New(30, 10)
	return DetailPaneModel{
		viewport: vp,
		overlay:  NewOverlay(),
		Mode:     DetailModeWelcome,
	}
}

// StartLoading puts the detail pane into an animated loading state.
func (m *DetailPaneModel) StartLoading(msg string) tea.Cmd {
	m.IsLoading = true
	m.overlay.Width = m.Width
	m.overlay.Height = m.Height
	return m.overlay.Show(msg)
}

// StopLoading clears the loading state.
func (m *DetailPaneModel) StopLoading() {
	m.IsLoading = false
	m.overlay.Hide()
}

func (m *DetailPaneModel) SetSize(width, height int) {
	m.Width = width
	m.Height = height
	m.viewport.Width = width - 4 // border + padding
	m.viewport.Height = height - 4
	m.updateViewportContent()
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

// SetSecretDiff displays a side-by-side comparison of one secret across two environments.
func (m *DetailPaneModel) SetSecretDiff(key, envA, envB, valueA, valueB string, missingA, missingB bool) {
	m.Mode = DetailModeDiff
	m.DiffKey = key
	m.DiffEnvA = envA
	m.DiffEnvB = envB
	m.DiffValueA = valueA
	m.DiffValueB = valueB
	m.DiffMissingA = missingA
	m.DiffMissingB = missingB
	m.DiffSelectedCol = 0
	m.ValueRevealed = false
	m.updateViewportContent()
}

// SetPropagation displays a secret's presence across all environments.
func (m *DetailPaneModel) SetPropagation(key string, currentEnv string, entries []PropagationEntry) {
	m.Mode = DetailModePropagation
	m.PropagationKey = key
	m.PropagationCurrentEnv = currentEnv
	m.PropagationEntries = entries
	m.PropagationSelectedCol = 0
	for i, e := range entries {
		if e.Env == currentEnv {
			m.PropagationSelectedCol = i
			break
		}
	}
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
		b.WriteString(fmt.Sprintf("Diff: %s\n", m.DiffKey))
		b.WriteString(fmt.Sprintf("%s: %s\n", m.DiffEnvA, m.DiffValueA))
		b.WriteString(fmt.Sprintf("%s: %s\n", m.DiffEnvB, m.DiffValueB))
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

	vpW := m.viewport.Width
	if vpW <= 0 {
		vpW = 40
	}

	// Label column is fixed; value wraps at the remaining width.
	const labelW = 12
	const indent = labelW + 2 // label + "  " spacer
	valueW := vpW - indent
	if valueW < 10 {
		valueW = 10
	}

	wrapValue := func(s string) string {
		return wordwrap.String(s, valueW)
	}

	b.WriteString(dTitleStyle.Render("Secret Detail"))
	b.WriteString("\n\n")

	b.WriteString(dLabelStyle.Render("Key:"))
	b.WriteString("  ")
	b.WriteString(dKeyStyle.Render(wrapValue(m.SecretKey)))
	b.WriteString("\n\n")

	b.WriteString(dLabelStyle.Render("Value:"))
	b.WriteString("  ")
	if m.ValueRevealed {
		b.WriteString(dValueStyle.Render(wrapValue(m.SecretValue)))
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
		b.WriteString(dValueStyle.Render(wrapValue(m.SecretComment)))
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

	vpW := m.viewport.Width
	if vpW <= 0 {
		vpW = 40
	}

	title := dTitleStyle.Render(m.OutputTitle)
	b.WriteString(title)
	b.WriteString("\n\n")

	wrapped := wordwrap.String(m.OutputContent, vpW)
	if m.OutputIsError {
		b.WriteString(dErrorStyle.Render(wrapped))
	} else {
		b.WriteString(dValueStyle.Render(wrapped))
	}

	return b.String()
}

func (m *DetailPaneModel) renderDiff() string {
	var b strings.Builder

	b.WriteString(dTitleStyle.Render("Secret Diff: " + m.DiffKey))
	b.WriteString("\n\n")

	bothExist := !m.DiffMissingA && !m.DiffMissingB
	valuesMatch := bothExist && m.DiffValueA == m.DiffValueB

	if valuesMatch {
		b.WriteString(dSuccessStyle.Render("= Values are identical"))
	} else if bothExist {
		b.WriteString(dChangedStyle.Render("~ Values differ"))
	}
	b.WriteString("\n\n")

	// colWidth is the inner width of each column cell (including 1-space padding each side)
	colWidth := (m.viewport.Width - 3) / 2
	if colWidth < 12 {
		colWidth = 12
	}
	innerWidth := colWidth - 2 // space for 1-char padding each side

	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))

	// cell pads content to colWidth with 1-space left indent, handles ANSI widths correctly
	cell := func(content string) string {
		return lipgloss.NewStyle().Width(colWidth).Render(" " + content)
	}

	row := func(left, right string) string {
		pipe := borderStyle.Render("│")
		return pipe + cell(left) + pipe + cell(right) + pipe
	}

	// Pre-wrap values to innerWidth
	wrapVal := func(value string, missing bool) []string {
		if missing {
			return []string{dErrorStyle.Render("(not set)")}
		}
		if !m.ValueRevealed {
			return []string{dSuccessStyle.Render("••••••••")}
		}
		var valStyle lipgloss.Style
		if valuesMatch {
			valStyle = dSuccessStyle
		} else {
			valStyle = dChangedStyle
		}
		lines := strings.Split(wordwrap.String(value, innerWidth-1), "\n")
		styled := make([]string, len(lines))
		for i, l := range lines {
			styled[i] = valStyle.Render(l)
		}
		return styled
	}

	linesA := wrapVal(m.DiffValueA, m.DiffMissingA)
	linesB := wrapVal(m.DiffValueB, m.DiffMissingB)

	// Pad both sides to the same number of lines
	for len(linesA) < len(linesB) {
		linesA = append(linesA, "")
	}
	for len(linesB) < len(linesA) {
		linesB = append(linesB, "")
	}

	dashes := strings.Repeat("─", colWidth)

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EAB308")).Bold(true)
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("#EAB308")).Foreground(lipgloss.Color("#111827")).Bold(true)

	envHeader := func(name string, col int) string {
		s := strings.ToUpper(name)
		if col == m.DiffSelectedCol {
			return selectedStyle.Render(s)
		}
		return headerStyle.Render(s)
	}

	// Top border
	b.WriteString(borderStyle.Render("┌" + dashes + "┬" + dashes + "┐"))
	b.WriteString("\n")

	// Tall header: empty padding + UPPERCASE env name + empty padding
	b.WriteString(row("", ""))
	b.WriteString("\n")
	b.WriteString(row(envHeader(m.DiffEnvA, 0), envHeader(m.DiffEnvB, 1)))
	b.WriteString("\n")
	b.WriteString(row("", ""))
	b.WriteString("\n")

	// Header/value divider
	b.WriteString(borderStyle.Render("├" + dashes + "┼" + dashes + "┤"))
	b.WriteString("\n")

	// Empty padding row
	b.WriteString(row("", ""))
	b.WriteString("\n")

	// Value rows
	for i := range linesA {
		b.WriteString(row(linesA[i], linesB[i]))
		b.WriteString("\n")
	}

	// Empty padding row
	b.WriteString(row("", ""))
	b.WriteString("\n")

	// Bottom border
	b.WriteString(borderStyle.Render("└" + dashes + "┴" + dashes + "┘"))
	b.WriteString("\n\n")

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	b.WriteString(dimStyle.Render("← → navigate   "))
	// Show copy hint if the selected column has a value to copy
	// Can copy here if the OTHER column has a value to copy from
	canCopy := (m.DiffSelectedCol == 0 && !m.DiffMissingB) || (m.DiffSelectedCol == 1 && !m.DiffMissingA)
	if canCopy {
		b.WriteString(dKeyStyle.Render("c") + dimStyle.Render(" copy here   "))
	}
	if m.ValueRevealed {
		b.WriteString(dMaskedStyle.Render("[press r to hide]"))
	} else {
		b.WriteString(dMaskedStyle.Render("[press r to reveal]"))
	}

	return b.String()
}

func (m *DetailPaneModel) renderPropagation() string {
	var b strings.Builder

	b.WriteString(dTitleStyle.Render("Secret Propagation: " + m.PropagationKey))
	b.WriteString("\n\n")

	entries := m.PropagationEntries
	n := len(entries)
	if n == 0 {
		b.WriteString(dValueStyle.Render("No environments found."))
		return b.String()
	}

	// Column width split evenly across N environments
	colWidth := (m.viewport.Width - n - 1) / n
	if colWidth < 12 {
		colWidth = 12
	}
	innerWidth := colWidth - 2

	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EAB308")).Bold(true)

	cell := func(content string) string {
		return lipgloss.NewStyle().Width(colWidth).Render(" " + content)
	}

	buildRow := func(cells []string) string {
		pipe := borderStyle.Render("│")
		var rb strings.Builder
		rb.WriteString(pipe)
		for _, c := range cells {
			rb.WriteString(cell(c))
			rb.WriteString(pipe)
		}
		return rb.String()
	}

	dashes := strings.Repeat("─", colWidth)
	dashesParts := make([]string, n)
	for i := range dashesParts {
		dashesParts[i] = dashes
	}

	topBorder := "┌" + strings.Join(dashesParts, "┬") + "┐"
	midBorder := "├" + strings.Join(dashesParts, "┼") + "┤"
	botBorder := "└" + strings.Join(dashesParts, "┴") + "┘"
	emptyCells := make([]string, n)

	// Top border
	b.WriteString(borderStyle.Render(topBorder))
	b.WriteString("\n")

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#EAB308")).
		Foreground(lipgloss.Color("#111827")).
		Bold(true)

	// Tall header: empty + UPPERCASE env name (marked if current) + empty
	b.WriteString(buildRow(emptyCells))
	b.WriteString("\n")
	headerCells := make([]string, n)
	for i, e := range entries {
		name := strings.ToUpper(e.Env)
		if e.Env == m.PropagationCurrentEnv {
			name += " *"
		}
		if i == m.PropagationSelectedCol {
			headerCells[i] = selectedStyle.Render(name)
		} else {
			headerCells[i] = headerStyle.Render(name)
		}
	}
	b.WriteString(buildRow(headerCells))
	b.WriteString("\n")
	b.WriteString(buildRow(emptyCells))
	b.WriteString("\n")

	// Header/value divider
	b.WriteString(borderStyle.Render(midBorder))
	b.WriteString("\n")

	// Pre-wrap values for each environment
	wrappedVals := make([][]string, n)
	maxLines := 0
	for i, e := range entries {
		if !e.Exists {
			wrappedVals[i] = []string{dErrorStyle.Render("(not set)")}
		} else if !m.ValueRevealed {
			wrappedVals[i] = []string{dSuccessStyle.Render("••••••••")}
		} else {
			var valStyle lipgloss.Style
			if e.MatchesCurrent || e.Env == m.PropagationCurrentEnv {
				valStyle = dSuccessStyle
			} else {
				valStyle = dChangedStyle
			}
			lines := strings.Split(wordwrap.String(e.Value, innerWidth-1), "\n")
			styled := make([]string, len(lines))
			for j, l := range lines {
				styled[j] = valStyle.Render(l)
			}
			wrappedVals[i] = styled
		}
		if len(wrappedVals[i]) > maxLines {
			maxLines = len(wrappedVals[i])
		}
	}
	for i := range wrappedVals {
		for len(wrappedVals[i]) < maxLines {
			wrappedVals[i] = append(wrappedVals[i], "")
		}
	}

	// Empty padding + value rows + empty padding
	b.WriteString(buildRow(emptyCells))
	b.WriteString("\n")
	for line := 0; line < maxLines; line++ {
		rowCells := make([]string, n)
		for i := range entries {
			rowCells[i] = wrappedVals[i][line]
		}
		b.WriteString(buildRow(rowCells))
		b.WriteString("\n")
	}
	b.WriteString(buildRow(emptyCells))
	b.WriteString("\n")

	// Bottom border
	b.WriteString(borderStyle.Render(botBorder))
	b.WriteString("\n\n")

	// Footer
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	b.WriteString(dimStyle.Render("← → navigate   * current env   "))
	selectedEntry := entries[m.PropagationSelectedCol]
	if selectedEntry.Env != m.PropagationCurrentEnv {
		b.WriteString(dKeyStyle.Render("c") + dimStyle.Render(" copy here   "))
	}
	if m.ValueRevealed {
		b.WriteString(dMaskedStyle.Render("[press r to hide]"))
	} else {
		b.WriteString(dMaskedStyle.Render("[press r to reveal]"))
	}

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
	b.WriteString(fmt.Sprintf("  %s  %s\n", dKeyStyle.Render("X"), "Delete secret"))
	b.WriteString(fmt.Sprintf("  %s  %s\n", dKeyStyle.Render("d"), "Compare secret across envs"))
	b.WriteString(fmt.Sprintf("  %s  %s\n", dKeyStyle.Render("p"), "Propagation across envs"))
	b.WriteString(fmt.Sprintf("  %s  %s\n", dKeyStyle.Render("?"), "Show all shortcuts"))

	return b.String()
}

func (m DetailPaneModel) Update(msg tea.Msg) (DetailPaneModel, tea.Cmd) {
	// Always handle overlay ticks regardless of active state
	if m.IsLoading {
		if _, ok := msg.(LoadingTickMsg); ok {
			var cmd tea.Cmd
			m.overlay, cmd = m.overlay.Update(msg)
			return m, cmd
		}
	}

	if !m.Active {
		return m, nil
	}

	if m.Mode == DetailModeDiff {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "left", "h":
				if m.DiffSelectedCol > 0 {
					m.DiffSelectedCol--
					m.updateViewportContent()
				}
				return m, nil
			case "right", "l":
				if m.DiffSelectedCol < 1 {
					m.DiffSelectedCol++
					m.updateViewportContent()
				}
				return m, nil
			case "c":
				// Selected column = destination; copy FROM the other env INTO it
				var value, targetEnv string
				if m.DiffSelectedCol == 0 && !m.DiffMissingB {
					// copy envB's value → envA
					value, targetEnv = m.DiffValueB, m.DiffEnvA
				} else if m.DiffSelectedCol == 1 && !m.DiffMissingA {
					// copy envA's value → envB
					value, targetEnv = m.DiffValueA, m.DiffEnvB
				}
				if targetEnv != "" {
					key := m.DiffKey
					return m, func() tea.Msg {
						return DiffCopyRequestMsg{Key: key, Value: value, TargetEnv: targetEnv}
					}
				}
				return m, nil
			}
		}
	}

	if m.Mode == DetailModePropagation {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "left", "h":
				if m.PropagationSelectedCol > 0 {
					m.PropagationSelectedCol--
					m.updateViewportContent()
				}
				return m, nil
			case "right", "l":
				if m.PropagationSelectedCol < len(m.PropagationEntries)-1 {
					m.PropagationSelectedCol++
					m.updateViewportContent()
				}
				return m, nil
			case "c":
				if m.PropagationSelectedCol < len(m.PropagationEntries) {
					entry := m.PropagationEntries[m.PropagationSelectedCol]
					if entry.Env != m.PropagationCurrentEnv {
						targetEnv := entry.Env
						return m, func() tea.Msg {
							return PropagationCopyRequestMsg{TargetEnv: targetEnv}
						}
					}
				}
				return m, nil
			}
		}
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

	if m.IsLoading {
		m.overlay.Width = m.Width
		m.overlay.Height = m.Height
		return style.Render(m.overlay.View())
	}

	m.updateViewportContent()
	return style.Render(m.viewport.View())
}

