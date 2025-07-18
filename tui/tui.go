package tui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ziyi233/onebot-tui/adapter"
	"github.com/ziyi233/onebot-tui/storage"
)

var (
	headerStyle      = lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230")).Padding(0, 1)
	statusStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Padding(0, 1)
	leftMsgStyle     = lipgloss.NewStyle().PaddingLeft(2)
	rightMsgStyle    = lipgloss.NewStyle().PaddingRight(2).Align(lipgloss.Right)
	senderStyle      = lipgloss.NewStyle().Bold(true)
	selfSenderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	cqImageRegex   = regexp.MustCompile(`\[CQ:image,.*?\]`)
	cqForwardRegex = regexp.MustCompile(`\[CQ:forward,.*?\]`)
)

// Model represents the state of the TUI.
type Model struct {
	appState    appState
	bot         adapter.BotAdapter
	store       *storage.Store
	messageChan chan adapter.Message

	viewport    viewport.Model
	textInput   textinput.Model
	headerText  string
	statusText  string
	activeChat  string
	messages    []adapter.Message
	ready       bool
}

// appState is an interface to get chat type without circular dependency
type appState interface {
	GetChatType(chatID string) string
	GetChatName(chatID string) string
}

// ActiveChatChangedMsg is a message to notify the TUI that the active chat has changed.
type ActiveChatChangedMsg struct {
	ID   string
	Name string
}

// CachesPopulatedMsg is a message to notify the TUI that the caches are populated.
type CachesPopulatedMsg struct{}

// New creates a new TUI model.
func New(appState appState, bot adapter.BotAdapter, store *storage.Store, messageChan chan adapter.Message) *Model {
	ti := textinput.New()
	ti.Placeholder = "Send a message..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 20

	return &Model{
		appState:    appState,
		bot:         bot,
		store:       store,
		messageChan: messageChan,
		textInput:   ti,
		headerText:  "No Active Chat",
		statusText:  "Ready. Press Ctrl+C to quit.",
		messages:    []adapter.Message{},
	}
}

// Init is the first command that is run when the program starts.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, waitForMessage(m.messageChan))
}

// Update handles all incoming messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := headerStyle.GetVerticalFrameSize()
		statusHeight := statusStyle.GetVerticalFrameSize()
		inputHeight := 1

		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerHeight - statusHeight - inputHeight
		m.textInput.Width = msg.Width - 2 // padding
		headerStyle.Width(msg.Width)
		statusStyle.Width(msg.Width)
		rightMsgStyle = rightMsgStyle.Width(m.viewport.Width)

		if !m.ready {
			m.ready = true
		}

		m.updateViewportContent()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			if m.activeChat != "" && m.textInput.Value() != "" {
				chatType := m.appState.GetChatType(m.activeChat)
				if chatType == "" {
					m.statusText = "Error: Unknown chat type."
				} else {
					messageContent := m.textInput.Value()
					err := m.bot.SendMessage(m.activeChat, chatType, messageContent)
					if err != nil {
						m.statusText = fmt.Sprintf("Error sending: %v", err)
					} else {
						sentMsg := adapter.Message{
							SenderName: "You",
							Content:    messageContent,
							Time:       time.Now(),
						}
						m.messages = append(m.messages, sentMsg)
						m.updateViewportContent()
						m.viewport.GotoBottom()
						m.textInput.Reset()
					}
				}
			}
		}

	case CachesPopulatedMsg:
		if m.activeChat != "" {
			chatName := m.appState.GetChatName(m.activeChat)
			if chatName != "" {
				m.headerText = fmt.Sprintf("Chat with %s", chatName)
			}
		}

	case ActiveChatChangedMsg:
		m.activeChat = msg.ID
		chatName := m.appState.GetChatName(msg.ID)
		if chatName == "" {
			chatName = msg.ID
		}
		m.headerText = fmt.Sprintf("Chat with %s", chatName)
		m.messages = []adapter.Message{}
		history, err := m.store.GetMessages(m.activeChat, 50)
		if err != nil {
			m.statusText = fmt.Sprintf("Error loading history: %v", err)
		} else {
			m.messages = history
		}
		m.updateViewportContent()
		m.viewport.GotoBottom()

	case adapter.Message:
		if msg.ChatID == m.activeChat {
			m.messages = append(m.messages, msg)
			m.updateViewportContent()
			m.viewport.GotoBottom()
		}
		return m, waitForMessage(m.messageChan)
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the TUI.
func (m *Model) View() string {
	if !m.ready {
		return "Initializing..."
	}
	return fmt.Sprintf("%s\n%s\n%s\n%s",
		m.headerView(),
		m.viewport.View(),
		m.textInput.View(),
		m.statusView(),
	)
}

func (m *Model) headerView() string {
	return headerStyle.Render(m.headerText)
}

func (m *Model) statusView() string {
	return statusStyle.Render(m.statusText)
}

func (m *Model) updateViewportContent() {
	var content strings.Builder
	for _, msg := range m.messages {
		var styledSender string
		var finalMsgStyle lipgloss.Style

		if msg.SenderName == "You" {
			styledSender = selfSenderStyle.Render(msg.SenderName)
			finalMsgStyle = rightMsgStyle
		} else {
			styledSender = senderStyle.Render(msg.SenderName)
			finalMsgStyle = leftMsgStyle
		}

		simplifiedContent := simplifyCQCodes(msg.Content)
		formattedMsg := fmt.Sprintf("%s\n%s", styledSender, simplifiedContent)
		content.WriteString(finalMsgStyle.Render(formattedMsg) + "\n")
	}
	m.viewport.SetContent(content.String())
}

func simplifyCQCodes(content string) string {
	content = cqImageRegex.ReplaceAllString(content, "[图片]")
	content = cqForwardRegex.ReplaceAllString(content, "[转发]")
	return content
}

// waitForMessage is a command that waits for a new message on the message channel.
func waitForMessage(ch chan adapter.Message) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}
