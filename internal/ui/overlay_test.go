package ui

import (
	"strings"
	"testing"
)

func TestOverlayLines_StyleLeak(t *testing.T) {
	// A simple base line with no formatting.
	base := "hello world"
	// An overlay line.
	overlay := "POP"

	// We overlay "POP" starting at index 6 (covering "wor").
	result := overlayLines(base, overlay, 6, 0)

	// The overlay text POP is expected to have the background style bgSeq.
	// But the trailing characters "ld" should have the style reset back to normal.
	// Specifically, we expect the reset sequence "\x1b[0m" to appear between POP and ld.
	expectedSuffix := "POP\x1b[0mld"
	if !strings.Contains(result, expectedSuffix) {
		t.Errorf("expected result to contain %q, but got %q", expectedSuffix, result)
	}
}
