package ui

import (
	"regexp"
	"strings"
)

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripAnsi(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

type cell struct {
	char  rune
	style string
}

func parseANSILine(line string) []cell {
	var cells []cell
	var currentStyle strings.Builder
	runes := []rune(line)
	inEscape := false

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\x1b' {
			inEscape = true
			currentStyle.WriteRune(r)
			continue
		}
		if inEscape {
			currentStyle.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		cells = append(cells, cell{
			char:  r,
			style: currentStyle.String(),
		})
	}
	return cells
}

func cellsToString(cells []cell) string {
	var sb strings.Builder
	var lastStyle string
	for _, c := range cells {
		if c.style != lastStyle {
			if lastStyle != "" {
				sb.WriteString("\x1b[0m")
			}
			sb.WriteString(c.style)
			lastStyle = c.style
		}
		sb.WriteRune(c.char)
	}
	sb.WriteString("\x1b[0m")
	return sb.String()
}

func overlayLines(base, overlay string, x, y int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	for i, oLine := range overlayLines {
		targetY := y + i
		if targetY < 0 || targetY >= len(baseLines) {
			continue
		}

		bLine := baseLines[targetY]
		bCells := parseANSILine(bLine)
		oCells := parseANSILine(oLine)

		if len(bCells) < x {
			padding := make([]cell, x-len(bCells))
			for p := range padding {
				padding[p] = cell{char: ' '}
			}
			bCells = append(bCells, padding...)
		}

		for j, oCell := range oCells {
			pos := x + j
			bgSeq := BgSeq
			if oCell.style == "" {
				oCell.style = bgSeq
			} else {
				oCell.style = strings.ReplaceAll(oCell.style, "\x1b[0m", "\x1b[0m"+bgSeq)
				oCell.style = bgSeq + oCell.style
			}

			if pos >= len(bCells) {
				bCells = append(bCells, oCell)
			} else {
				bCells[pos] = oCell
			}
		}

		baseLines[targetY] = cellsToString(bCells)
	}

	return strings.Join(baseLines, "\n")
}
