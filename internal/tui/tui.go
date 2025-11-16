package tui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/brojonat/context-vacuum/internal/parser"
	"github.com/brojonat/context-vacuum/internal/storage"
	"github.com/brojonat/context-vacuum/internal/storage/dbgen"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginLeft(2)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("170")).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginLeft(2)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			MarginLeft(2)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			MarginLeft(2)

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("170")).
			Padding(1, 2).
			Width(60)

	inputLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)
)

type model struct {
	store      *storage.Store
	parser     *parser.Parser
	logger     *slog.Logger
	sources    []dbgen.Source
	cursor     int
	message    string
	err        error

	// Add mode fields
	addMode    bool
	nameInput  textinput.Model
	pathInput  textinput.Model
	focusIndex int // 0 = name, 1 = path

	// Delete confirmation
	deleteConfirm bool
	deleteTarget  string
}

func initialModel(store *storage.Store, parser *parser.Parser, logger *slog.Logger) (model, error) {
	ctx := context.Background()
	sources, err := store.Queries().ListSources(ctx)
	if err != nil {
		return model{}, err
	}

	// Initialize text inputs
	nameInput := textinput.New()
	nameInput.Placeholder = "e.g., API Documentation"
	nameInput.Focus()
	nameInput.CharLimit = 100
	nameInput.Width = 50

	pathInput := textinput.New()
	pathInput.Placeholder = "e.g., /path/to/file.md or https://..."
	pathInput.CharLimit = 500
	pathInput.Width = 50

	return model{
		store:         store,
		parser:        parser,
		logger:        logger,
		sources:       sources,
		cursor:        0,
		addMode:       false,
		nameInput:     nameInput,
		pathInput:     pathInput,
		focusIndex:    0,
		deleteConfirm: false,
		deleteTarget:  "",
	}, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle add mode separately
	if m.addMode {
		return m.updateAddMode(msg)
	}

	// Handle delete confirmation mode
	if m.deleteConfirm {
		return m.updateDeleteConfirm(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "a":
			// Enter add mode
			m.addMode = true
			m.nameInput.Focus()
			m.pathInput.Blur()
			m.nameInput.SetValue("")
			m.pathInput.SetValue("")
			m.focusIndex = 0
			m.message = ""
			return m, nil

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.sources)-1 {
				m.cursor++
			}

		case " ", "enter":
			// Toggle enabled state
			if len(m.sources) > 0 {
				source := m.sources[m.cursor]
				newEnabled := int64(1)
				if source.Enabled == 1 {
					newEnabled = 0
				}

				ctx := context.Background()
				err := m.store.Queries().UpdateSourceEnabled(ctx, dbgen.UpdateSourceEnabledParams{
					Enabled: newEnabled,
					Name:    source.Name,
				})

				if err != nil {
					m.message = fmt.Sprintf("Error: %v", err)
				} else {
					// Reload sources
					sources, err := m.store.Queries().ListSources(ctx)
					if err != nil {
						m.message = fmt.Sprintf("Error reloading: %v", err)
					} else {
						m.sources = sources
						status := "disabled"
						if newEnabled == 1 {
							status = "enabled"
						}
						m.message = fmt.Sprintf("Toggled %s to %s", source.Name, status)
					}
				}
			}

		case "d":
			// Delete current source (with confirmation)
			if len(m.sources) > 0 {
				source := m.sources[m.cursor]
				m.deleteConfirm = true
				m.deleteTarget = source.Name
				m.message = ""
			}

		case "r":
			// Reload sources
			ctx := context.Background()
			sources, err := m.store.Queries().ListSources(ctx)
			if err != nil {
				m.message = fmt.Sprintf("Error reloading: %v", err)
			} else {
				m.sources = sources
				m.message = "Sources reloaded"
			}
		}
	}

	return m, cmd
}

func (m model) updateAddMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "esc":
			// Exit add mode
			m.addMode = false
			m.message = "Add cancelled"
			return m, nil

		case "tab", "shift+tab":
			// Switch focus between inputs
			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % 2
			} else {
				m.focusIndex = (m.focusIndex - 1 + 2) % 2
			}

			if m.focusIndex == 0 {
				m.nameInput.Focus()
				m.pathInput.Blur()
			} else {
				m.nameInput.Blur()
				m.pathInput.Focus()
			}
			return m, nil

		case "enter":
			// Submit the form
			name := strings.TrimSpace(m.nameInput.Value())
			path := strings.TrimSpace(m.pathInput.Value())

			// Validate
			if name == "" {
				m.message = "Error: Name cannot be empty"
				return m, nil
			}
			if path == "" {
				m.message = "Error: Path cannot be empty"
				return m, nil
			}

			// Add the source
			ctx := context.Background()
			if err := m.addSource(ctx, name, path); err != nil {
				m.message = fmt.Sprintf("Error: %v", err)
				return m, nil
			}

			// Reload sources
			sources, err := m.store.Queries().ListSources(ctx)
			if err != nil {
				m.message = fmt.Sprintf("Error reloading: %v", err)
			} else {
				m.sources = sources
				m.message = fmt.Sprintf("✓ Added source: %s", name)
			}

			// Exit add mode
			m.addMode = false
			return m, nil
		}
	}

	// Update the focused input
	if m.focusIndex == 0 {
		m.nameInput, cmd = m.nameInput.Update(msg)
	} else {
		m.pathInput, cmd = m.pathInput.Update(msg)
	}

	return m, cmd
}

func (m model) updateDeleteConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "y", "Y":
			// Confirm deletion
			ctx := context.Background()
			err := m.store.Queries().DeleteSource(ctx, m.deleteTarget)
			if err != nil {
				m.message = fmt.Sprintf("Error deleting: %v", err)
			} else {
				// Reload sources
				sources, err := m.store.Queries().ListSources(ctx)
				if err != nil {
					m.message = fmt.Sprintf("Error reloading: %v", err)
				} else {
					m.sources = sources
					// Adjust cursor if needed
					if m.cursor >= len(m.sources) && m.cursor > 0 {
						m.cursor = len(m.sources) - 1
					}
					m.message = fmt.Sprintf("✓ Deleted source: %s", m.deleteTarget)
				}
			}
			m.deleteConfirm = false
			m.deleteTarget = ""
			return m, nil

		case "n", "N", "esc":
			// Cancel deletion
			m.deleteConfirm = false
			m.deleteTarget = ""
			m.message = "Delete cancelled"
			return m, nil
		}
	}

	return m, nil
}

// addSource adds a new source to the database
func (m model) addSource(ctx context.Context, name, path string) error {
	// Determine source type and parse content
	var content string
	var sourceType string
	var err error

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		sourceType = "url"
		content, err = m.parser.ParseURL(path)
		if err != nil {
			return fmt.Errorf("failed to fetch URL: %w", err)
		}
	} else {
		sourceType = "file"
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to resolve path: %w", err)
		}
		path = absPath
		content, err = m.parser.ParseFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
	}

	// Compute hash
	hash := storage.ComputeHash(content)

	// Check if already exists
	existing, err := m.store.Queries().GetSourceByName(ctx, name)
	if err == nil {
		// Source exists, update it
		return m.store.Queries().UpdateSourceContent(ctx, dbgen.UpdateSourceContentParams{
			Content: content,
			Hash:    hash,
			ID:      existing.ID,
		})
	}

	// Create new source (enabled by default)
	_, err = m.store.Queries().CreateSource(ctx, dbgen.CreateSourceParams{
		Name:       name,
		SourceType: sourceType,
		Path:       path,
		Content:    content,
		Hash:       hash,
		Enabled:    1,
	})

	return err
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🚀 context-vacuum - Interactive Source Manager"))
	b.WriteString("\n\n")

	// Show add modal if in add mode
	if m.addMode {
		b.WriteString(m.renderAddModal())
		return b.String()
	}

	// Show delete confirmation modal
	if m.deleteConfirm {
		b.WriteString(m.renderDeleteConfirm())
		return b.String()
	}

	// Normal source list view
	if len(m.sources) == 0 {
		b.WriteString(normalItemStyle.Render("  No sources found. Press 'a' to add sources."))
		b.WriteString("\n\n")
	} else {
		for i, source := range m.sources {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			enabled := "[ ]"
			if source.Enabled == 1 {
				enabled = "[✓]"
			}

			style := normalItemStyle
			if i == m.cursor {
				style = selectedItemStyle
			}

			line := fmt.Sprintf("%s %s %s (%s)",
				cursor,
				enabled,
				truncate(source.Name, 40),
				source.SourceType,
			)

			b.WriteString(style.Render(line))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	if m.message != "" {
		msgStyle := statusStyle
		if strings.HasPrefix(m.message, "Error") {
			msgStyle = errorStyle
		}
		b.WriteString(msgStyle.Render(m.message))
		b.WriteString("\n\n")
	}

	help := "a: add • d: delete • ↑/k: up • ↓/j: down • space/enter: toggle • r: reload • q: quit"
	b.WriteString(helpStyle.Render(help))
	b.WriteString("\n")

	return b.String()
}

func (m model) renderAddModal() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(inputLabelStyle.Render("Name:"))
	b.WriteString("\n")
	b.WriteString(m.nameInput.View())
	b.WriteString("\n\n")

	b.WriteString(inputLabelStyle.Render("Path or URL:"))
	b.WriteString("\n")
	b.WriteString(m.pathInput.View())
	b.WriteString("\n\n")

	help := helpStyle.Render("[Enter] Add  [Tab] Switch  [Esc] Cancel")
	b.WriteString(help)

	if m.message != "" {
		b.WriteString("\n\n")
		msgStyle := statusStyle
		if strings.HasPrefix(m.message, "Error") {
			msgStyle = errorStyle
		}
		b.WriteString(msgStyle.Render(m.message))
	}

	modal := modalStyle.Render(b.String())
	return modal
}

func (m model) renderDeleteConfirm() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(errorStyle.Render(fmt.Sprintf("⚠️  Delete source: %s?", m.deleteTarget)))
	b.WriteString("\n\n")
	b.WriteString(normalItemStyle.Render("This action cannot be undone."))
	b.WriteString("\n\n")

	help := helpStyle.Render("[Y] Yes, delete  [N/Esc] Cancel")
	b.WriteString(help)

	modal := modalStyle.Render(b.String())
	return modal
}

// Run starts the TUI
func Run(store *storage.Store) error {
	// Create parser and logger for add functionality
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Quiet during TUI
	}))
	parser := parser.NewParser(10 * 1024 * 1024) // 10MB default

	m, err := initialModel(store, parser, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize TUI: %w", err)
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
