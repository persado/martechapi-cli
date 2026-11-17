package editor

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func key(s string) tea.KeyPressMsg {
	switch s {
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "ctrl+r":
		return tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

func send(m model, msgs ...tea.Msg) (model, tea.Cmd) {
	var cmd tea.Cmd
	for _, msg := range msgs {
		var next tea.Model
		next, cmd = m.Update(msg)
		m = next.(model)
	}
	return m, cmd
}

func ok(string) error { return nil }

func TestStripComments(t *testing.T) {
	tests := []struct{ in, want string }{
		{"// a\n{}\n", "{}"},
		{"  // indented\n{\"u\": \"http://x\"}", `{"u": "http://x"}`},
		{"// only\n// comments", ""},
		{"", ""},
	}
	for _, tc := range tests {
		if got := StripComments(tc.in); got != tc.want {
			t.Errorf("StripComments(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSaveValid(t *testing.T) {
	m, cmd := send(newModel("T", "// help\n{}", ok), key("ctrl+s"))
	if !m.saved || m.result != "{}" || cmd == nil {
		t.Fatalf("saved=%v result=%q cmd=%v", m.saved, m.result, cmd)
	}
}

func TestSaveInvalidStaysOpen(t *testing.T) {
	bad := func(string) error { return errors.New("account.name: required") }
	m, cmd := send(newModel("T", "{}", bad), key("ctrl+s"))
	if m.saved || cmd != nil {
		t.Fatal("invalid text must not quit")
	}
	if !strings.Contains(m.View().Content, "account.name: required") {
		t.Error("view does not show the validation error")
	}
	m, _ = send(m, key("x"))
	if m.errMsg != "" {
		t.Error("typing must clear the error")
	}
}

func TestCancel(t *testing.T) {
	t.Run("unchanged quits at once", func(t *testing.T) {
		_, cmd := send(newModel("T", "{}", ok), key("esc"))
		if cmd == nil {
			t.Fatal("esc on an unchanged buffer must quit")
		}
	})
	t.Run("changed asks first", func(t *testing.T) {
		m, cmd := send(newModel("T", "{}", ok), key("x"), key("esc"))
		if cmd != nil || !m.quitting {
			t.Fatal("esc on a changed buffer must ask for confirmation")
		}
		if !strings.Contains(m.View().Content, "Discard your changes") {
			t.Error("confirmation prompt missing")
		}
		m, cmd = send(m, key("n"))
		if m.quitting || cmd != nil {
			t.Fatal("any key other than Y keeps editing")
		}
		_, cmd = send(m, key("esc"), key("y"))
		if cmd == nil {
			t.Fatal("Y must quit")
		}
	})
}

func TestReset(t *testing.T) {
	m, _ := send(newModel("T", "{}", ok), key("x"), key("ctrl+r"))
	if m.ta.Value() != "{}" {
		t.Errorf("value after reset = %q", m.ta.Value())
	}
}

func TestViewShowsKeys(t *testing.T) {
	m, _ := send(newModel("Score request", "{}", ok), tea.WindowSizeMsg{Width: 120, Height: 30})
	v := m.View()
	for _, want := range []string{"Score request", "Ctrl+S", "Esc", "Ctrl+R", "Ln 1, Col 1"} {
		if !strings.Contains(v.Content, want) {
			t.Errorf("view lacks %q", want)
		}
	}
	if !v.AltScreen {
		t.Error("editor must use the alternate screen")
	}
}

func TestEdit(t *testing.T) {
	opts := []tea.ProgramOption{tea.WithWindowSize(80, 24), tea.WithoutSignals()}
	t.Run("save", func(t *testing.T) {
		got, err := Edit(context.Background(), "T", "// c\n{\"a\":1}", ok, strings.NewReader("\x13"), io.Discard, opts...)
		if err != nil || got != `{"a":1}` {
			t.Fatalf("Edit = %q, %v", got, err)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		_, err := Edit(context.Background(), "T", "{}", ok, strings.NewReader("\x1b"), io.Discard, opts...)
		if !errors.Is(err, ErrCanceled) {
			t.Fatalf("err = %v, want ErrCanceled", err)
		}
	})
}
