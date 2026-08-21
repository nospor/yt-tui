package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
)

func textareaCursorOffset(ta textarea.Model) int {
	val := ta.Value()
	if val == "" {
		return 0
	}
	lines := strings.Split(val, "\n")
	row := ta.Line()
	if row < 0 {
		row = 0
	}
	if row >= len(lines) {
		row = len(lines) - 1
	}
	offset := 0
	for i := 0; i < row; i++ {
		offset += len([]rune(lines[i])) + 1
	}
	offset += ta.LineInfo().CharOffset
	return offset
}

func detectMentionQuery(ta textarea.Model) (active bool, start int, query string) {
	val := ta.Value()
	if val == "" {
		return false, 0, ""
	}
	cursorOffset := textareaCursorOffset(ta)
	runes := []rune(val)
	if cursorOffset > len(runes) {
		cursorOffset = len(runes)
	}
	for i := cursorOffset - 1; i >= 0; i-- {
		if runes[i] == '@' {
			if i > 0 {
				prev := runes[i-1]
				if prev != ' ' && prev != '\n' && prev != '\t' {
					return false, 0, ""
				}
			}
			queryRunes := runes[i+1 : cursorOffset]
			for _, r := range queryRunes {
				if r == ' ' || r == '\n' || r == '\t' {
					return false, 0, ""
				}
			}
			return true, i, string(queryRunes)
		}
		if runes[i] == ' ' || runes[i] == '\n' || runes[i] == '\t' {
			return false, 0, ""
		}
	}
	return false, 0, ""
}

func insertMentionInTextarea(ta textarea.Model, mentionStart int, login string) textarea.Model {
	val := ta.Value()
	cursorOffset := textareaCursorOffset(ta)
	runes := []rune(val)
	if mentionStart < 0 || mentionStart > cursorOffset || cursorOffset > len(runes) {
		return ta
	}
	replacement := "@" + login + " "
	newRunes := append(append(append([]rune{}, runes[:mentionStart]...), []rune(replacement)...), runes[cursorOffset:]...)
	newCursor := mentionStart + len([]rune(replacement))
	ta.SetValue(string(newRunes))
	textareaSetCursorOffset(&ta, newCursor)
	return ta
}

func textareaSetCursorOffset(ta *textarea.Model, target int) {
	val := ta.Value()
	runes := []rune(val)
	if target < 0 {
		target = 0
	}
	if target > len(runes) {
		target = len(runes)
	}
	line := 0
	col := 0
	for i := 0; i < target && i < len(runes); i++ {
		if runes[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	ta.CursorStart()
	for i := 0; i < line; i++ {
		ta.CursorDown()
	}
	ta.SetCursor(col)
}
