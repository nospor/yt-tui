package ui

import (
	"regexp"
	"testing"

	"github.com/charmbracelet/bubbletea"
)

type dummyModel struct {
	slice []string
}

func (m dummyModel) Update() dummyModel {
	m.slice = append(m.slice, "hello")
	return m
}

func TestSliceValueReceiver(t *testing.T) {
	m := dummyModel{}
	if len(m.slice) != 0 {
		t.Errorf("expected 0, got %d", len(m.slice))
	}

	m = m.Update()
	if len(m.slice) != 1 {
		t.Errorf("expected 1, got %d", len(m.slice))
	}

	m = m.Update()
	if len(m.slice) != 2 {
		t.Errorf("expected 2, got %d", len(m.slice))
	}
}

func TestCommentPasteScreenshotsIncremental(t *testing.T) {
	// Mock getClipboardImage to return dummy data
	oldGet := getClipboardImage
	getClipboardImage = func() ([]byte, string, error) {
		return []byte("fake-image"), "image/png", nil
	}
	defer func() {
		getClipboardImage = oldGet
	}()

	m := newDetailModel(nil, nil)
	m.loading = false // MUST clear loading to process keys!
	m.mode = modeCommentInput
	m.commentInput.SetValue("thi ")
	m.commentInput.SetCursor(4)

	// Simulate first paste
	vMsg := tea.KeyMsg{Type: tea.KeyCtrlV}
	m, _ = m.Update(vMsg)

	val1 := m.commentInput.Value()
	re1 := regexp.MustCompile(`^thi !\[pasted-image-\d{8}-\d{6}-1\.png\]\(pasted-image-\d{8}-\d{6}-1\.png\)$`)
	if !re1.MatchString(val1) {
		t.Errorf("value '%s' does not match pattern", val1)
	}
	if len(m.pastedCommentImages) != 1 {
		t.Errorf("expected 1 image, got %d", len(m.pastedCommentImages))
	}

	// Type a space and "2 "
	m.commentInput.SetValue(val1 + " 2 ")
	m.commentInput.SetCursor(len([]rune(m.commentInput.Value())))

	// Simulate second paste
	m, _ = m.Update(vMsg)

	val2 := m.commentInput.Value()
	re2 := regexp.MustCompile(`^thi !\[pasted-image-\d{8}-\d{6}-1\.png\]\(pasted-image-\d{8}-\d{6}-1\.png\) 2 !\[pasted-image-\d{8}-\d{6}-2\.png\]\(pasted-image-\d{8}-\d{6}-2\.png\)$`)
	if !re2.MatchString(val2) {
		t.Errorf("value '%s' does not match pattern", val2)
	}
	if len(m.pastedCommentImages) != 2 {
		t.Errorf("expected 2 images, got %d", len(m.pastedCommentImages))
	}
}

func TestFormPasteScreenshotsIncremental(t *testing.T) {
	// Mock getClipboardImage to return dummy data
	oldGet := getClipboardImage
	getClipboardImage = func() ([]byte, string, error) {
		return []byte("fake-image"), "image/png", nil
	}
	defer func() {
		getClipboardImage = oldGet
	}()

	m := newFormModel(nil, nil)
	m.loading = false
	m.focusIndex = fieldDescription
	m.descTextArea.SetValue("thi ")
	m.descTextArea.SetCursor(4)

	// Simulate first paste
	vMsg := tea.KeyMsg{Type: tea.KeyCtrlV}
	m, _ = m.Update(vMsg)

	val1 := m.descTextArea.Value()
	re1 := regexp.MustCompile(`^thi !\[pasted-image-\d{8}-\d{6}-1\.png\]\(pasted-image-\d{8}-\d{6}-1\.png\)$`)
	if !re1.MatchString(val1) {
		t.Errorf("value '%s' does not match pattern", val1)
	}
	if len(m.pastedImages) != 1 {
		t.Errorf("expected 1 image, got %d", len(m.pastedImages))
	}

	// Type a space and "2 "
	m.descTextArea.SetValue(val1 + " 2 ")
	m.descTextArea.SetCursor(len([]rune(m.descTextArea.Value())))

	// Simulate second paste
	m, _ = m.Update(vMsg)

	val2 := m.descTextArea.Value()
	re2 := regexp.MustCompile(`^thi !\[pasted-image-\d{8}-\d{6}-1\.png\]\(pasted-image-\d{8}-\d{6}-1\.png\) 2 !\[pasted-image-\d{8}-\d{6}-2\.png\]\(pasted-image-\d{8}-\d{6}-2\.png\)$`)
	if !re2.MatchString(val2) {
		t.Errorf("value '%s' does not match pattern", val2)
	}
	if len(m.pastedImages) != 2 {
		t.Errorf("expected 2 images, got %d", len(m.pastedImages))
	}
}
