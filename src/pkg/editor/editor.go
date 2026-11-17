// Package editor is the built-in full-screen editor behind --edit. It does not
// start an external program, so it works the same on every OS.
package editor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ErrCanceled is returned when the user leaves the editor without saving.
var ErrCanceled = errors.New("edit canceled")

// Validator checks the edited text, with comment lines already removed.
type Validator func(string) error

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("24")).Padding(0, 1)
	keyStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("250")).Padding(0, 1)
	helpStyle  = lipgloss.NewStyle().Faint(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	askStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
)

const (
	headerLines = 2
	footerLines = 2
)

var keyBar = strings.Join([]string{
	keyStyle.Render("Ctrl+S") + " save and run",
	keyStyle.Render("Esc") + " cancel",
	keyStyle.Render("Ctrl+R") + " reset template",
	keyStyle.Render("Ctrl+K") + " delete to end of line",
}, "   ")

type model struct {
	title    string
	template string
	validate Validator
	ta       textarea.Model
	errMsg   string
	result   string
	saved    bool
	quitting bool
}

func newModel(title, text string, validate Validator) model {
	ta := textarea.New()
	ta.ShowLineNumbers = true
	ta.MaxHeight = 0
	ta.MaxWidth = 0
	ta.KeyMap.Paste.SetEnabled(false)
	ta.SetValue(text)
	ta.MoveToBegin()
	ta.Focus()
	return model{title: title, template: text, validate: validate, ta: ta}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.ta.SetWidth(msg.Width)
		m.ta.SetHeight(max(1, msg.Height-headerLines-footerLines))
		return m, nil
	case tea.KeyPressMsg:
		if m.quitting {
			switch msg.String() {
			case "y", "Y", "esc", "ctrl+c", "ctrl+q":
				return m, tea.Quit
			}
			m.quitting = false
			return m, nil
		}
		switch msg.String() {
		case "ctrl+s":
			text := StripComments(m.ta.Value())
			if err := m.validate(text); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			m.result, m.saved = text, true
			return m, tea.Quit
		case "esc", "ctrl+c", "ctrl+q":
			if m.ta.Value() == m.template {
				return m, tea.Quit
			}
			m.quitting = true
			return m, nil
		case "ctrl+r":
			m.ta.SetValue(m.template)
			m.ta.MoveToBegin()
			m.errMsg = ""
			return m, nil
		}
		m.errMsg = ""
	}
	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)
	return m, cmd
}

func (m model) View() tea.View {
	info := helpStyle.Render(fmt.Sprintf("Edit the JSON below. Lines starting with // are help and are ignored.   Ln %d, Col %d",
		m.ta.Line()+1, m.ta.Column()+1))
	status := ""
	switch {
	case m.quitting:
		status = askStyle.Render("Discard your changes and cancel? Press Y to discard, any other key to keep editing.")
	case m.errMsg != "":
		status = errorStyle.Render("✗ Not saved: " + strings.ReplaceAll(m.errMsg, "\n", " · ") + "   Fix it and press Ctrl+S again.")
	}
	v := tea.NewView(titleStyle.Render(m.title) + "\n" + info + "\n" + m.ta.View() + "\n" + status + "\n" + keyBar)
	v.AltScreen = true
	v.Cursor = m.ta.Cursor()
	if v.Cursor != nil {
		v.Cursor.Y += headerLines
	}
	return v
}

// Edit shows text in the editor until the user saves text that validate
// accepts, and returns that text without comment lines. Leaving without
// saving returns ErrCanceled.
func Edit(ctx context.Context, title, text string, validate Validator, in io.Reader, out io.Writer, opts ...tea.ProgramOption) (string, error) {
	opts = append([]tea.ProgramOption{tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out)}, opts...)
	final, err := tea.NewProgram(newModel(title, text, validate), opts...).Run()
	if err != nil {
		return "", err
	}
	m := final.(model)
	if !m.saved {
		return "", ErrCanceled
	}
	return m.result, nil
}

// StripComments removes the lines whose first non-blank characters are //.
func StripComments(s string) string {
	lines := strings.Split(s, "\n")
	kept := lines[:0]
	for _, l := range lines {
		if !strings.HasPrefix(strings.TrimSpace(l), "//") {
			kept = append(kept, l)
		}
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}
