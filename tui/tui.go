package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// ConnectionInfo holds credentials for a single SSH hop.
type ConnectionInfo struct {
	Username  string `json:"username"`
	IPAddress string `json:"ip_address"`
	Port      string `json:"port"`
	Password  string `json:"password"`
}

// SavedConnection is a source + optional single nested hop, persisted to disk.
type SavedConnection struct {
	Source ConnectionInfo  `json:"source"`
	Nested *ConnectionInfo `json:"nested,omitempty"`
}

const divider = "  ──────────────────────────────"

type item struct {
	title string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return "" }
func (i item) FilterValue() string { return i.title }

type pastItem struct {
	conn SavedConnection
}

func (p pastItem) Title() string {
	return fmt.Sprintf("%s@%s:%s", p.conn.Source.Username, p.conn.Source.IPAddress, p.conn.Source.Port)
}

func (p pastItem) Description() string {
	if p.conn.Nested != nil {
		return fmt.Sprintf("-> %s@%s:%s", p.conn.Nested.Username, p.conn.Nested.IPAddress, p.conn.Nested.Port)
	}
	return "direct connection"
}

func (p pastItem) FilterValue() string { return p.Title() }

type state int

const (
	stateList state = iota
	stateInput
	stateAddNested
	statePastList
)

type inputField int

const (
	fieldUsername inputField = iota
	fieldIPAddress
	fieldPort
	fieldPassword
)

type Model struct {
	list           list.Model
	pastList       list.Model
	usernameInput  textinput.Model
	passwordInput  textinput.Model
	ipaddressInput textinput.Model
	portInput      textinput.Model
	state          state
	activeField    inputField
	enteringNested bool
	Username       string
	IPAddress      string
	Port           string
	Password       string
	Connections    []ConnectionInfo
}

func InitModel() Model {
	items := []list.Item{
		item{title: "Connect to Server"},
		item{title: "Past Connections"},
		item{title: "Configuration"},
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false

	l := list.New(items, delegate, 30, 14)
	l.Title = "Nest"
	l.SetShowStatusBar(false)

	usernameInput := textinput.New()
	usernameInput.Prompt = "  > "
	usernameInput.Placeholder = ""
	usernameInput.CharLimit = 256
	usernameInput.SetWidth(40)

	ipaddressInput := textinput.New()
	ipaddressInput.Prompt = "  > "
	ipaddressInput.Placeholder = ""
	ipaddressInput.CharLimit = 256
	ipaddressInput.SetWidth(40)

	portInput := textinput.New()
	portInput.Prompt = "  > "
	portInput.Placeholder = "22"
	portInput.CharLimit = 5
	portInput.SetWidth(40)

	passwordInput := textinput.New()
	passwordInput.Prompt = "  > "
	passwordInput.Placeholder = ""
	passwordInput.CharLimit = 256
	passwordInput.SetWidth(40)
	passwordInput.EchoMode = textinput.EchoPassword
	passwordInput.EchoCharacter = '*'

	return Model{
		list:           l,
		usernameInput:  usernameInput,
		ipaddressInput: ipaddressInput,
		passwordInput:  passwordInput,
		portInput:      portInput,
		state:          stateList,
		activeField:    fieldUsername,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch m.state {
		case stateList:
			if msg.String() == "enter" {
				selected := m.list.SelectedItem()
				if selected == nil {
					break
				}
				switch selected.(item).title {
				case "Connect to Server":
					m.state = stateInput
					m.activeField = fieldUsername
					m.enteringNested = false
					m.Connections = nil
					return m, m.usernameInput.Focus()
				case "Past Connections":
					entries := loadSavedConnections()
					items := make([]list.Item, len(entries))
					for i, e := range entries {
						items[i] = pastItem{conn: e}
					}
					delegate := list.NewDefaultDelegate()
					delegate.ShowDescription = true
					m.pastList = list.New(items, delegate, 40, 14)
					m.pastList.Title = "Past Connections"
					m.pastList.SetShowStatusBar(false)
					m.state = statePastList
					return m, nil
				}
			}

		case stateInput:
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				switch m.activeField {
				case fieldPassword:
					m.activeField = fieldPort
					m.passwordInput.Blur()
					return m, m.portInput.Focus()
				case fieldPort:
					m.activeField = fieldIPAddress
					m.portInput.Blur()
					return m, m.ipaddressInput.Focus()
				case fieldIPAddress:
					m.activeField = fieldUsername
					m.ipaddressInput.Blur()
					return m, m.usernameInput.Focus()
				default:
					m.clearInputs()
					if m.enteringNested {
						m.state = stateAddNested
						return m, nil
					}
					m.state = stateList
					return m, nil
				}
			case "enter":
				switch m.activeField {
				case fieldUsername:
					m.Username = m.usernameInput.Value()
					m.activeField = fieldIPAddress
					m.usernameInput.Blur()
					return m, m.ipaddressInput.Focus()
				case fieldIPAddress:
					m.IPAddress = m.ipaddressInput.Value()
					m.activeField = fieldPort
					m.ipaddressInput.Blur()
					return m, m.portInput.Focus()
				case fieldPort:
					m.Port = m.portInput.Value()
					if m.Port == "" {
						m.Port = "22"
					}
					m.activeField = fieldPassword
					m.portInput.Blur()
					return m, m.passwordInput.Focus()
				case fieldPassword:
					m.Password = m.passwordInput.Value()
					m.Connections = append(m.Connections, ConnectionInfo{
						Username:  m.Username,
						IPAddress: m.IPAddress,
						Port:      m.Port,
						Password:  m.Password,
					})
					m.clearInputs()
					if m.enteringNested {
						// Max 1 nested — done
						return m, tea.Quit
					}
					m.state = stateAddNested
					return m, nil
				}
			}

		case stateAddNested:
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "y", "Y":
				m.state = stateInput
				m.activeField = fieldUsername
				m.enteringNested = true
				return m, m.usernameInput.Focus()
			case "n", "N":
				return m, tea.Quit
			case "esc":
				if len(m.Connections) > 0 {
					last := m.Connections[len(m.Connections)-1]
					m.Connections = m.Connections[:len(m.Connections)-1]
					m.Username = last.Username
					m.IPAddress = last.IPAddress
					m.Port = last.Port
					m.Password = last.Password
					m.usernameInput.SetValue(last.Username)
					m.ipaddressInput.SetValue(last.IPAddress)
					m.portInput.SetValue(last.Port)
					m.passwordInput.SetValue(last.Password)
					m.state = stateInput
					m.activeField = fieldPassword
					return m, m.passwordInput.Focus()
				}
				m.state = stateList
				return m, nil
			}

		case statePastList:
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "enter":
				selected := m.pastList.SelectedItem()
				if pi, ok := selected.(pastItem); ok {
					m.Connections = []ConnectionInfo{pi.conn.Source}
					if pi.conn.Nested != nil {
						m.Connections = append(m.Connections, *pi.conn.Nested)
					}
					return m, tea.Quit
				}
			case "esc":
				m.state = stateList
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height)
		if m.state == statePastList {
			m.pastList.SetWidth(msg.Width)
			m.pastList.SetHeight(msg.Height)
		}
		return m, nil
	}

	var cmd tea.Cmd
	switch m.state {
	case stateList:
		m.list, cmd = m.list.Update(msg)
	case statePastList:
		m.pastList, cmd = m.pastList.Update(msg)
	case stateInput:
		switch m.activeField {
		case fieldUsername:
			m.usernameInput, cmd = m.usernameInput.Update(msg)
		case fieldIPAddress:
			m.ipaddressInput, cmd = m.ipaddressInput.Update(msg)
		case fieldPort:
			m.portInput, cmd = m.portInput.Update(msg)
		case fieldPassword:
			m.passwordInput, cmd = m.passwordInput.Update(msg)
		}
	}
	return m, cmd
}

func (m *Model) clearInputs() {
	m.usernameInput.Reset()
	m.ipaddressInput.Reset()
	m.portInput.Reset()
	m.passwordInput.Reset()
	m.Username = ""
	m.IPAddress = ""
	m.Port = ""
	m.Password = ""
}

func (m Model) View() tea.View {
	switch m.state {
	case stateInput:
		return tea.NewView(m.viewInput())
	case stateAddNested:
		return tea.NewView(m.viewAddNested())
	case statePastList:
		return tea.NewView(m.pastList.View())
	default:
		return tea.NewView(m.list.View())
	}
}

func (m Model) viewInput() string {
	var sb strings.Builder

	subtitle := "New Connection"
	if m.enteringNested {
		subtitle = "Nested Connection"
	}
	fmt.Fprintf(&sb, "\n  Nest > %s\n\n", subtitle)

	// Show source info when entering nested credentials
	if m.enteringNested && len(m.Connections) > 0 {
		src := m.Connections[0]
		fmt.Fprintf(&sb, "  Source  %s@%s:%s\n", src.Username, src.IPAddress, src.Port)
		sb.WriteString(divider + "\n\n")
	}

	// Show previously entered fields for current hop
	switch m.activeField {
	case fieldUsername:
		sb.WriteString("  Username\n")
		sb.WriteString(m.usernameInput.View() + "\n")
	case fieldIPAddress:
		fmt.Fprintf(&sb, "  Username    %s\n", m.Username)
		sb.WriteString("  IP Address\n")
		sb.WriteString(m.ipaddressInput.View() + "\n")
	case fieldPort:
		fmt.Fprintf(&sb, "  Username    %s\n", m.Username)
		fmt.Fprintf(&sb, "  IP Address  %s\n", m.IPAddress)
		sb.WriteString("  Port\n")
		sb.WriteString(m.portInput.View() + "\n")
	case fieldPassword:
		fmt.Fprintf(&sb, "  Username    %s\n", m.Username)
		fmt.Fprintf(&sb, "  IP Address  %s\n", m.IPAddress)
		fmt.Fprintf(&sb, "  Port        %s\n", m.Port)
		sb.WriteString("  Password\n")
		sb.WriteString(m.passwordInput.View() + "\n")
	}

	sb.WriteString("\n  enter next  |  esc back\n")
	return sb.String()
}

func (m Model) viewAddNested() string {
	var sb strings.Builder
	sb.WriteString("\n  Nest > Connection Ready\n\n")

	if len(m.Connections) > 0 {
		src := m.Connections[0]
		fmt.Fprintf(&sb, "  Source  %s@%s:%s\n", src.Username, src.IPAddress, src.Port)
	}

	sb.WriteString(divider + "\n\n")
	sb.WriteString("  Add a nested connection? (y/n)\n")
	sb.WriteString("\n  y nest  |  n connect  |  esc back\n")
	return sb.String()
}

func (m Model) GetConnections() []ConnectionInfo {
	return m.Connections
}

func (m Model) GetUsername() string {
	if len(m.Connections) > 0 {
		return m.Connections[0].Username
	}
	return ""
}

func (m Model) GetIPAddress() string {
	if len(m.Connections) > 0 {
		return m.Connections[0].IPAddress
	}
	return ""
}

func (m Model) GetPort() string {
	if len(m.Connections) > 0 {
		p := m.Connections[0].Port
		if p == "" {
			return "22"
		}
		return p
	}
	return "22"
}

func (m Model) GetPassword() string {
	if len(m.Connections) > 0 {
		return m.Connections[0].Password
	}
	return ""
}

// --- persistence ---

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "nest", "connections.json")
}

func loadSavedConnections() []SavedConnection {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil
	}
	var conns []SavedConnection
	if err := json.Unmarshal(data, &conns); err != nil {
		return nil
	}
	return conns
}

func saveConnectionToHistory(conn SavedConnection) {
	conns := loadSavedConnections()
	found := false
	for i, c := range conns {
		if sameConnection(c, conn) {
			conns[i] = conn
			found = true
			break
		}
	}
	if !found {
		conns = append(conns, conn)
	}
	data, err := json.MarshalIndent(conns, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(configPath()), 0700)
	_ = os.WriteFile(configPath(), data, 0600)
}

func sameConnection(a, b SavedConnection) bool {
	if a.Source.Username != b.Source.Username ||
		a.Source.IPAddress != b.Source.IPAddress ||
		a.Source.Port != b.Source.Port {
		return false
	}
	if (a.Nested == nil) != (b.Nested == nil) {
		return false
	}
	if a.Nested != nil {
		return a.Nested.Username == b.Nested.Username &&
			a.Nested.IPAddress == b.Nested.IPAddress &&
			a.Nested.Port == b.Nested.Port
	}
	return true
}

func RunTui() Model {
	p := tea.NewProgram(InitModel())
	m, err := p.Run()
	if err != nil {
		fmt.Printf("Error detected: %v", err)
		os.Exit(1)
	}
	result := m.(Model)
	if len(result.Connections) > 0 {
		saved := SavedConnection{Source: result.Connections[0]}
		if len(result.Connections) > 1 {
			nested := result.Connections[1]
			saved.Nested = &nested
		}
		saveConnectionToHistory(saved)
	}
	return result
}
