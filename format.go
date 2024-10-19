package main

import (
	"bufio"
	"bytes"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"os"
	"os/exec"
	"strings"
)

// Parse ANSI code string and update the current style
func parseANSICode(code string, style TextStyle) TextStyle {
	parts := strings.Split(code, ";")
	for _, part := range parts {
		switch part {
		case "0":
			// Reset
			style = TextStyle{}
		case "1":
			// Bold
			style.Bold = true
		case "4":
			// Underline
			style.Underline = true
		case "30":
			style.Foreground = tcell.ColorBlack
		case "31":
			style.Foreground = tcell.ColorMaroon
		case "32":
			style.Foreground = tcell.ColorGreen
		case "33":
			style.Foreground = tcell.ColorOlive
		case "34":
			style.Foreground = tcell.ColorNavy
		case "35":
			style.Foreground = tcell.ColorPurple
		case "36":
			style.Foreground = tcell.ColorTeal
		case "37":
			style.Foreground = tcell.ColorSilver
		// Add more color codes as needed
		default:
			// Ignore unsupported codes
		}
	}
	return style
}

// Process ANSI escape sequences and return a slice of ColData
func processANSIStrings(s string) []ColData {
	var cols []ColData
	var currentStyle TextStyle
	var textBuilder strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+2 < len(s) && s[i+1] == '[' {
			// Flush current text
			if textBuilder.Len() > 0 {
				cols = append(cols, ColData{
					Text:  textBuilder.String(),
					Style: currentStyle,
				})
				textBuilder.Reset()
			}
			// Parse escape sequence
			seqEnd := strings.Index(s[i:], "m")
			if seqEnd == -1 {
				break
			}
			seq := s[i+2 : i+seqEnd]
			currentStyle = parseANSICode(seq, currentStyle)
			i += seqEnd + 1
		} else {
			textBuilder.WriteByte(s[i])
			i++
		}
	}
	// Append remaining text
	if textBuilder.Len() > 0 {
		cols = append(cols, ColData{
			Text:  textBuilder.String(),
			Style: currentStyle,
		})
	}
	return cols
}

// Helper function to render markdown output including ANSI escape sequences
func renderMarkdown(x, y int, content []byte) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	row := y
	for scanner.Scan() {
		line := scanner.Text()
		cols := processANSIStrings(line)
		col := x
		for _, colData := range cols {
			style := tcell.StyleDefault
			if colData.Style.Bold {
				style = style.Bold(true)
			}
			if colData.Style.Underline {
				style = style.Underline(true)
			}
			style = style.Foreground(colData.Style.Foreground)
			style = style.Background(colData.Style.Background)
			for _, r := range colData.Text {
				screen.SetContent(col, row, r, nil, style)
				col += runewidth.RuneWidth(r)
			}
		}
		row++
	}
}

// Struct to hold text and style after processing ANSI escape sequences
type ColData struct {
	Text  string
	Style TextStyle
}

type TextStyle struct {
	Bold       bool
	Underline  bool
	Foreground tcell.Color
	Background tcell.Color
}

func formatTreeItem(item TreeItem) string {
	var builder strings.Builder

	for i := 0; i < len(item.Prefixes)-1; i++ {
		if item.Prefixes[i] {
			builder.WriteString("│   ")
		} else {
			builder.WriteString("    ")
		}
	}

	if len(item.Prefixes) > 0 {
		if item.Prefixes[len(item.Prefixes)-1] {
			builder.WriteString("├── ")
		} else {
			builder.WriteString("└── ")
		}
	}

	builder.WriteString(item.Display)
	return builder.String()
}

func resetTerminal() {
	cmd := exec.Command("stty", "sane")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}
