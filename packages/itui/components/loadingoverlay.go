package components

import (
	"strings"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LoadingTickMsg drives the loading overlay animation.
type LoadingTickMsg struct{ id int }

const overlayFPS = time.Second / 14

var overlayIDCounter int64

func nextOverlayID() int {
	return int(atomic.AddInt64(&overlayIDCounter, 1))
}

// OverlayModel is a full-pane Infisical-branded loading animation.
type OverlayModel struct {
	id      int
	frame   int
	Visible bool
	Message string
	Width   int
	Height  int
}

func NewOverlay() OverlayModel {
	return OverlayModel{id: nextOverlayID()}
}

// Show activates the overlay with the given message and kicks off animation.
func (m *OverlayModel) Show(msg string) tea.Cmd {
	m.Visible = true
	m.Message = msg
	m.frame = 0
	return m.tick()
}

// Hide deactivates the overlay.
func (m *OverlayModel) Hide() {
	m.Visible = false
}

func (m OverlayModel) tick() tea.Cmd {
	id := m.id
	return tea.Tick(overlayFPS, func(_ time.Time) tea.Msg {
		return LoadingTickMsg{id: id}
	})
}

// Update handles tick messages and advances the animation frame.
func (m OverlayModel) Update(msg tea.Msg) (OverlayModel, tea.Cmd) {
	if !m.Visible {
		return m, nil
	}
	if tick, ok := msg.(LoadingTickMsg); ok && tick.id == m.id {
		m.frame++
		return m, m.tick()
	}
	return m, nil
}

// View renders the full-pane loading animation.
func (m OverlayModel) View() string {
	if !m.Visible {
		return ""
	}

	innerW := m.Width - 6
	if innerW < 24 {
		innerW = 24
	}
	innerH := m.Height - 4
	if innerH < 10 {
		innerH = 10
	}

	// ── Title ────────────────────────────────────────────────────
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EAB308")).
		Bold(true)
	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true)

	// Pulse the title brightness between two shades on every 6 frames
	titleText := "✦  I N F I S I C A L  ✦"
	if (m.frame/6)%2 == 1 {
		titleStyle = titleStyle.Foreground(lipgloss.Color("#FDE68A"))
	}

	title := titleStyle.Width(innerW).Align(lipgloss.Center).Render(titleText)
	subtitle := subtitleStyle.Width(innerW).Align(lipgloss.Center).Render("secrets manager")

	// ── Lock icon ────────────────────────────────────────────────
	keyholes := []string{"○", "◌", "●", "◉", "●", "◌"}
	keyhole := keyholes[m.frame%len(keyholes)]

	lockBorderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EAB308"))
	keyholeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EAB308")).
		Bold(true)

	lockTop := lockBorderStyle.Render("╭──────╮")
	lockMid := lockBorderStyle.Render("│") + "      " + lockBorderStyle.Render("│")
	lockBody := lockBorderStyle.Render("╔══╧══╧══╗")
	lockKey := lockBorderStyle.Render("║") + "  " + keyholeStyle.Render(keyhole) + "     " + lockBorderStyle.Render("║")
	lockBot := lockBorderStyle.Render("╚═════════╝")

	lockLines := []string{lockTop, lockMid, lockBody, lockKey, lockBot}
	var lockBlock strings.Builder
	for _, l := range lockLines {
		lockBlock.WriteString(
			lipgloss.NewStyle().Width(innerW).Align(lipgloss.Center).Render(l) + "\n",
		)
	}

	// ── Scan bar ─────────────────────────────────────────────────
	barInnerW := innerW - 4
	if barInnerW < 10 {
		barInnerW = 10
	}

	// Bounce the comet position back and forth
	period := 2 * (barInnerW - 1)
	rawPos := m.frame % period
	cometPos := rawPos
	if rawPos >= barInnerW {
		cometPos = period - rawPos
	}

	scanLine := buildGlowBar(barInnerW, cometPos)

	barFrameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))
	barTop := barFrameStyle.Render("╔") + strings.Repeat("═", barInnerW) + barFrameStyle.Render("╗")
	barMid := barFrameStyle.Render("║") + scanLine + barFrameStyle.Render("║")
	barBot := barFrameStyle.Render("╚") + strings.Repeat("═", barInnerW) + barFrameStyle.Render("╝")

	barBlock := lipgloss.NewStyle().Width(innerW).Align(lipgloss.Center).Render(barTop) + "\n" +
		lipgloss.NewStyle().Width(innerW).Align(lipgloss.Center).Render(barMid) + "\n" +
		lipgloss.NewStyle().Width(innerW).Align(lipgloss.Center).Render(barBot)

	// ── Spinner + message with animated dots ─────────────────────
	spinFrames := []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
	spinFrame := spinFrames[m.frame%len(spinFrames)]

	dotSuffix := []string{"   ", ".  ", ".. ", "..."}
	dot := dotSuffix[(m.frame/4)%len(dotSuffix)]

	spinStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EAB308")).Bold(true)
	msgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F9FAFB"))

	spinLine := spinStyle.Render(spinFrame) + "  " + msgStyle.Render(m.Message+dot)
	spinCentered := lipgloss.NewStyle().Width(innerW).Align(lipgloss.Center).Render(spinLine)

	// ── Vertical centering ────────────────────────────────────────
	contentLines := 5 + 1 + 1 + 3 + 1 + 1 // lock(5) + blank + subtitle+title(2) + bar(3) + blank + spin(1)
	topPadLines := (innerH - contentLines) / 2
	if topPadLines < 1 {
		topPadLines = 1
	}
	topPad := strings.Repeat("\n", topPadLines)

	return topPad +
		title + "\n" +
		subtitle + "\n\n" +
		lockBlock.String() +
		barBlock + "\n\n" +
		spinCentered
}

// buildGlowBar builds a single scan-bar line with a glowing comet at cometPos.
func buildGlowBar(width, cometPos int) string {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#1F2937"))
	trailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#92400E"))
	midStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706"))
	brightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FDE68A"))

	var b strings.Builder
	for i := 0; i < width; i++ {
		switch {
		case i == cometPos+2:
			b.WriteString(brightStyle.Render("█"))
		case i == cometPos+1:
			b.WriteString(brightStyle.Render("▓"))
		case i == cometPos:
			b.WriteString(midStyle.Render("▒"))
		case i == cometPos-1:
			b.WriteString(trailStyle.Render("░"))
		default:
			b.WriteString(dimStyle.Render("░"))
		}
	}
	return b.String()
}
