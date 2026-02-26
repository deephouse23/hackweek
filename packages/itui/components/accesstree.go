package components

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Access level ────────────────────────────────────────────────────────────

// AccessLevel represents the permission level for a single action on a folder.
type AccessLevel int

const (
	AccessLevelUnknown AccessLevel = iota // no rule data loaded
	AccessLevelNone                       // explicitly no permission
	AccessLevelPartial                    // permitted under conditions only
	AccessLevelFull                       // unconditionally permitted
)

// ─── Tree data ───────────────────────────────────────────────────────────────

// AccessTreeNode is a folder node with computed permission levels per action.
//
// Secret actions keyed in Actions:
//
//	"describe"     – list/view secret metadata
//	"read-value"   – view secret value
//	"create"       – create a secret
//	"edit"         – update a secret
//	"delete"       – delete a secret
//
// Folder actions keyed in Actions:
//
//	"folder-create" – create a subfolder
//	"folder-edit"   – rename / update a folder
//	"folder-delete" – delete a folder
type AccessTreeNode struct {
	Name     string
	Path     string // full path from root, e.g. "/app/config"
	Depth    int
	Children []*AccessTreeNode
	Actions  map[string]AccessLevel
}

// ─── Messages ────────────────────────────────────────────────────────────────

// AccessTreeCloseMsg is sent when the user dismisses the access tree.
type AccessTreeCloseMsg struct{}

// AccessTreeEnvChangedMsg is sent when the user switches environment tabs.
type AccessTreeEnvChangedMsg struct{ Env string }

// AccessTreeDataMsg carries the fully-built tree (or an error) back to the model.
type AccessTreeDataMsg struct {
	Nodes []*AccessTreeNode
	Err   error
}

// ─── Pagination ──────────────────────────────────────────────────────────────

const atPageSize = 10

// ─── Styles ──────────────────────────────────────────────────────────────────

var (
	atBorderColor = lipgloss.Color("#EAB308") // Infisical yellow
	atAccentColor = lipgloss.Color("#FACC15") // lighter yellow
	atGreenColor  = lipgloss.Color("#10B981")
	atYellowColor = lipgloss.Color("#F59E0B")
	atRedColor    = lipgloss.Color("#EF4444")
	atTextColor   = lipgloss.Color("#F9FAFB")
	atDimColor    = lipgloss.Color("#6B7280")
	atBgColor     = lipgloss.Color("#1F2937")
)

// ─── Model ───────────────────────────────────────────────────────────────────

// AccessTreeModel is the Bubble Tea component for the access tree view.
type AccessTreeModel struct {
	Visible bool
	width   int
	height  int

	Environments  []string
	envIdx        int // current environment tab index
	Identity      string
	Nodes         []*AccessTreeNode
	Loading       bool
	Err           error

	viewport    viewport.Model
	filterInput textinput.Model
	FilterActive bool

	// shownMap tracks per-node-path how many children to display (pagination).
	shownMap map[string]int
}

func NewAccessTree() AccessTreeModel {
	ti := textinput.New()
	ti.Placeholder = "secret name..."
	ti.CharLimit = 64
	ti.Width = 28

	vp := viewport.New(80, 20)

	return AccessTreeModel{
		filterInput: ti,
		viewport:    vp,
		shownMap:    make(map[string]int),
	}
}

// Show opens the access tree overlay and initialises it for a fresh data load.
func (m *AccessTreeModel) Show(envs []string, currentEnv, identity string) {
	m.Visible = true
	m.Environments = envs
	m.Identity = identity
	m.Loading = true
	m.Err = nil
	m.Nodes = nil
	m.FilterActive = false
	m.filterInput.SetValue("")
	m.shownMap = make(map[string]int)

	m.envIdx = 0
	for i, e := range envs {
		if e == currentEnv {
			m.envIdx = i
			break
		}
	}
	m.viewport.GotoTop()
}

// Hide dismisses the overlay.
func (m *AccessTreeModel) Hide() {
	m.Visible = false
	m.FilterActive = false
	m.filterInput.Blur()
}

// CurrentEnv returns the slug of the currently-selected environment tab.
func (m AccessTreeModel) CurrentEnv() string {
	if m.envIdx >= 0 && m.envIdx < len(m.Environments) {
		return m.Environments[m.envIdx]
	}
	return ""
}

// SetSize tells the component about available terminal space.
func (m *AccessTreeModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	// Reserve rows for title, divider, tabs, legend, filter, footer, borders/padding.
	vpH := h - 16
	if vpH < 4 {
		vpH = 4
	}
	// Width(m.width-2) frame + Padding(1,3) + double border = m.width-12 usable inner.
	vpW := w - 12
	if vpW < 20 {
		vpW = 20
	}
	m.viewport.Width = vpW
	m.viewport.Height = vpH
}

// ─── Update ──────────────────────────────────────────────────────────────────

func (m AccessTreeModel) Update(msg tea.Msg) (AccessTreeModel, tea.Cmd) {
	if !m.Visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Filter-input mode eats most keys.
		if m.FilterActive {
			switch msg.String() {
			case "esc", "enter":
				m.FilterActive = false
				m.filterInput.Blur()
				m.rebuildView()
				return m, nil
			}
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.rebuildView()
			return m, cmd
		}

		switch msg.String() {
		case "esc", "A", "a":
			m.Hide()
			return m, func() tea.Msg { return AccessTreeCloseMsg{} }

		case "tab":
			if len(m.Environments) > 1 {
				m.envIdx = (m.envIdx + 1) % len(m.Environments)
				env := m.CurrentEnv()
				m.Loading = true
				m.Nodes = nil
				return m, func() tea.Msg { return AccessTreeEnvChangedMsg{Env: env} }
			}

		case "shift+tab":
			if len(m.Environments) > 1 {
				n := len(m.Environments)
				m.envIdx = (m.envIdx - 1 + n) % n
				env := m.CurrentEnv()
				m.Loading = true
				m.Nodes = nil
				return m, func() tea.Msg { return AccessTreeEnvChangedMsg{Env: env} }
			}

		case "/":
			m.FilterActive = true
			m.filterInput.Focus()
			return m, nil
		}

	case AccessTreeDataMsg:
		m.Loading = false
		m.Nodes = msg.Nodes
		m.Err = msg.Err
		m.viewport.GotoTop()
		m.rebuildView()
		return m, nil
	}

	// Forward remaining events to viewport for scroll support.
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// ─── View ────────────────────────────────────────────────────────────────────

func (m AccessTreeModel) View() string {
	if !m.Visible {
		return ""
	}

	dimS := lipgloss.NewStyle().Foreground(atDimColor)
	boldYellow := lipgloss.NewStyle().Foreground(atBorderColor).Bold(true)

	// Inner usable width: frame Width(m.width-2) minus 2 borders minus 2*3 padding.
	innerW := m.width - 12
	if innerW < 20 {
		innerW = 20
	}

	// Title row
	title := boldYellow.Render("  Access Tree")
	closeHint := dimS.Render("Esc · close  ")
	titleLine := padBetween(title, closeHint, innerW)
	divider := dimS.Render(strings.Repeat("─", innerW))

	// Environment tabs
	envLine := "  " + m.renderEnvTabs()

	// Legend — just the color key; action names appear inline in the tree rows.
	legend := "  " +
		lipgloss.NewStyle().Foreground(atGreenColor).Render("●") +
		dimS.Render("  Full access      ") +
		lipgloss.NewStyle().Foreground(atYellowColor).Render("◑") +
		dimS.Render("  Partial / conditional      ") +
		lipgloss.NewStyle().Foreground(atRedColor).Render("○") +
		dimS.Render("  No access")

	// Filter bar
	var filterLine string
	if m.FilterActive {
		filterLine = "  " + dimS.Render("Filter: ") + m.filterInput.View()
	} else {
		filterLine = "  " + dimS.Render("Press  /  to filter by secret name")
	}

	// Main body
	var body string
	if m.Loading {
		body = "\n" + lipgloss.NewStyle().Foreground(atBorderColor).Render("    ⟳  Loading access tree...") + "\n"
	} else if m.Err != nil {
		body = "\n" + lipgloss.NewStyle().Foreground(atRedColor).Render("    Error: "+m.Err.Error()) + "\n"
	} else {
		body = m.viewport.View()
	}

	// Footer
	scrollHint := ""
	if !m.Loading && m.viewport.TotalLineCount() > m.viewport.Height {
		scrollHint = dimS.Render(fmt.Sprintf("   %d%%", int(m.viewport.ScrollPercent()*100)))
	}
	footer := "  " + dimS.Render("↑ ↓  scroll     Tab / Shift+Tab  switch env     /  filter     Esc  close") + scrollHint

	inner := lipgloss.JoinVertical(lipgloss.Left,
		titleLine,
		divider,
		"",
		envLine,
		"",
		legend,
		"",
		filterLine,
		"",
		body,
		"",
		footer,
	)

	frame := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(atBorderColor).
		Background(atBgColor).
		Padding(1, 3).
		Width(m.width - 2)

	return frame.Render(inner)
}

// ─── Internal rendering ───────────────────────────────────────────────────────

func (m *AccessTreeModel) rebuildView() {
	m.viewport.SetContent(m.renderTree())
}

func (m AccessTreeModel) renderTree() string {
	if len(m.Nodes) == 0 {
		return lipgloss.NewStyle().Foreground(atDimColor).Render("  (no folders found)")
	}

	filter := m.filterInput.Value()

	var sb strings.Builder

	// Root identity node
	rootS := lipgloss.NewStyle().Foreground(atAccentColor).Bold(true)
	identity := m.Identity
	if identity == "" {
		identity = "current identity"
	}
	sb.WriteString(rootS.Render("◆  "+identity) + "\n")

	for i, node := range m.Nodes {
		isLast := i == len(m.Nodes)-1
		m.renderNode(&sb, node, filter, isLast, "")
	}

	return sb.String()
}

func (m AccessTreeModel) renderNode(sb *strings.Builder, node *AccessTreeNode, filter string, isLast bool, prefix string) {
	connStyle := lipgloss.NewStyle().Foreground(atDimColor)
	pathStyle := lipgloss.NewStyle().Foreground(atTextColor).Bold(true)
	folderIcon := lipgloss.NewStyle().Foreground(atYellowColor).Render("📁")

	connector := "├─"
	childPfx := prefix + "│  "
	if isLast {
		connector = "└─"
		childPfx = prefix + "   "
	}

	// ── folder name line ──
	sb.WriteString(
		prefix + connStyle.Render(connector) + " " + folderIcon + " " +
			pathStyle.Render(node.Name) + "\n",
	)

	// ── action badges line ──
	actionIndent := prefix + connStyle.Render(strings.TrimRight(childPfx[len(prefix):], " ")) + "   "
	if isLast {
		actionIndent = prefix + "       "
	}
	sb.WriteString(actionIndent + m.renderBadges(node, filter) + "\n")
	sb.WriteString("\n")

	// ── children (paginated) ──
	shown := m.shownMap[node.Path]
	if shown == 0 {
		shown = atPageSize
	}
	total := len(node.Children)
	// Sort children alphabetically for stable ordering.
	sorted := make([]*AccessTreeNode, len(node.Children))
	copy(sorted, node.Children)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	for i, child := range sorted {
		if i >= shown {
			break
		}
		childIsLast := (i == total-1) || (i == shown-1 && shown >= total)
		m.renderNode(sb, child, filter, childIsLast, childPfx)
	}

	if shown < total {
		remaining := total - shown
		moreS := lipgloss.NewStyle().Foreground(atDimColor).Italic(true)
		sb.WriteString(childPfx + moreS.Render(fmt.Sprintf("  ▸ show %d more…", remaining)) + "\n")
	}
}

// renderBadges returns the labeled action-dot line for a node.
//
// Format:  list ●   read ●   add ●   edit ●   del ●     │     mkdir ●   mv ●   rmdir ●
func (m AccessTreeModel) renderBadges(node *AccessTreeNode, filter string) string {
	type actionDef struct {
		key   string
		label string
	}
	secretDefs := []actionDef{
		{"describe", "list"},
		{"read-value", "read"},
		{"create", "add"},
		{"edit", "edit"},
		{"delete", "del"},
	}
	folderDefs := []actionDef{
		{"folder-create", "mkdir"},
		{"folder-edit", "mv"},
		{"folder-delete", "rmdir"},
	}

	dimS := lipgloss.NewStyle().Foreground(atDimColor)

	resolveLevel := func(action string, level AccessLevel) AccessLevel {
		if filter != "" && action == "read-value" && level == AccessLevelPartial {
			return AccessLevelFull
		}
		return level
	}

	var secretParts []string
	for _, def := range secretDefs {
		level := resolveLevel(def.key, node.Actions[def.key])
		secretParts = append(secretParts, dimS.Render(def.label)+" "+renderDot(level))
	}

	var folderParts []string
	for _, def := range folderDefs {
		folderParts = append(folderParts, dimS.Render(def.label)+" "+renderDot(node.Actions[def.key]))
	}

	sep := dimS.Render("   │   ")
	return strings.Join(secretParts, "   ") + sep + strings.Join(folderParts, "   ")
}

func renderDot(level AccessLevel) string {
	switch level {
	case AccessLevelFull:
		return lipgloss.NewStyle().Foreground(atGreenColor).Render("●")
	case AccessLevelPartial:
		return lipgloss.NewStyle().Foreground(atYellowColor).Render("◑")
	case AccessLevelNone:
		return lipgloss.NewStyle().Foreground(atRedColor).Render("○")
	default:
		return lipgloss.NewStyle().Foreground(atDimColor).Render("·")
	}
}

func (m AccessTreeModel) renderEnvTabs() string {
	if len(m.Environments) == 0 {
		return ""
	}
	var parts []string
	for i, env := range m.Environments {
		if i == m.envIdx {
			tab := lipgloss.NewStyle().
				Background(atBorderColor).
				Foreground(lipgloss.Color("#111827")).
				Bold(true).
				Padding(0, 1).
				Render(env)
			parts = append(parts, tab)
		} else {
			tab := lipgloss.NewStyle().
				Foreground(atDimColor).
				Padding(0, 1).
				Render(env)
			parts = append(parts, tab)
		}
	}
	return strings.Join(parts, " ")
}

// padBetween inserts spaces between left and right so together they fill width.
func padBetween(left, right string, width int) string {
	lw := lipgloss.Width(left)
	rw := lipgloss.Width(right)
	spaces := width - lw - rw
	if spaces < 1 {
		spaces = 1
	}
	return left + strings.Repeat(" ", spaces) + right
}
