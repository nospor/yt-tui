package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFilePickerZoxideOpenAndClose(t *testing.T) {
	m := newDetailModel(nil, nil)
	m.width = 80
	m.height = 24
	m.filepickerActive = true

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if cmd == nil {
		t.Fatal("expected load cmd")
	}
	if !m.zoxide.active {
		t.Fatal("expected zoxide mode")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.zoxide.active {
		t.Fatal("expected zoxide mode off")
	}
	if !m.filepickerActive {
		t.Fatal("file picker should stay open")
	}
}

func TestFormFilePickerZoxideOpen(t *testing.T) {
	m := newFormModel(nil, nil)
	m.width = 80
	m.height = 24
	m.filepickerActive = true

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if cmd == nil {
		t.Fatal("expected load cmd")
	}
	if !m.zoxide.active {
		t.Fatal("expected zoxide mode")
	}
}

func TestJumpFilePickerToDirectory(t *testing.T) {
	dir := t.TempDir()
	fp := newZoxideOverlay()
	picker := newDetailModel(nil, nil).filepicker

	res := applyFilepickerZoxide(fp, picker, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	res.Overlay.paths = []string{dir}
	res.Overlay.loading = false
	res = applyFilepickerZoxide(res.Overlay, res.Picker, tea.KeyMsg{Type: tea.KeyEnter})
	if !res.Jumped {
		t.Fatal("expected jump")
	}
	if res.Overlay.active {
		t.Fatal("zoxide overlay should close")
	}
	if res.Picker.CurrentDirectory != dir {
		t.Fatalf("directory = %q", res.Picker.CurrentDirectory)
	}
}

func TestJumpFilePickerToMissingDirectory(t *testing.T) {
	z := newZoxideOverlay()
	z.active = true
	z.paths = []string{filepath.Join(t.TempDir(), "missing")}
	picker := newDetailModel(nil, nil).filepicker

	res := applyFilepickerZoxide(z, picker, tea.KeyMsg{Type: tea.KeyEnter})
	if res.Jumped {
		t.Fatal("expected no jump")
	}
	if !res.Overlay.active {
		t.Fatal("overlay should stay open on error")
	}
	if res.Overlay.errText == "" {
		t.Fatal("expected error message")
	}
}

func TestZoxideLoadedMsgIgnoredWhenClosed(t *testing.T) {
	z := newZoxideOverlay()
	z = z.handleLoaded(zoxideDirsLoadedMsg{Paths: []string{"/tmp"}})
	if z.active {
		t.Fatal("should stay closed")
	}
	if len(z.paths) != 0 {
		t.Fatalf("paths = %v", z.paths)
	}
}

func TestZoxideLoadedMsgStaleQueryIgnored(t *testing.T) {
	z := newZoxideOverlay()
	z.active = true
	z.input.SetValue("foo")
	z = z.handleLoaded(zoxideDirsLoadedMsg{Query: "bar", Paths: []string{"/tmp"}})
	if len(z.paths) != 0 {
		t.Fatalf("stale paths applied: %v", z.paths)
	}
}

func TestZoxideOverlayRender(t *testing.T) {
	z := newZoxideOverlay()
	z.active = true
	z.paths = []string{"/tmp/alpha", "/tmp/beta"}
	view := z.render(80, 24)
	if !strings.Contains(view, "Jump with zoxide") {
		t.Fatalf("missing title: %q", view)
	}
	if !strings.Contains(view, "/tmp/alpha") {
		t.Fatalf("missing path: %q", view)
	}
}
