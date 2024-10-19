package main

import (
	markdown "github.com/MichaelMure/go-term-markdown"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"os"
)

// Render a line of text on the screen at a given x, y position
func renderText(x, y int, text string, style tcell.Style) {
	col := x
	for _, r := range text {
		screen.SetContent(col, y, r, nil, style)
		col += runewidth.RuneWidth(r)
	}
}

// Render footer with key hints
func renderFooter(selectedItem TreeItem) {
	width, height := screen.Size()
	hint := ""
	if isFile(selectedItem.Path) {
		hint = "E: Edit | D: Delete | M: Move | Q: Quit"
	} else {
		hint = "N: New Dir/File | E: Rename | D: Delete | Q: Quit"
	}
	renderClearArea(0, height-1, width, height)
	renderText(0, height-1, hint, tcell.StyleDefault)
}

// Render markdown preview for files
func renderMarkdownPreview(path string, startX int) {
	width, height := screen.Size()
	if isFile(path) {
		source, err := os.ReadFile(path)
		if err != nil {
			return
		}
		lines := markdown.Render(string(source), height-2, 0)
		// Clear previous preview content
		renderClearArea(startX, 0, width, height-2)
		renderMarkdown(startX, 1, lines)
	} else {
		// Clear preview area if not a file
		renderClearArea(startX, 0, width, height-2)
	}
}

// Render the directory tree and highlight the current selection
func renderTree(tree []TreeItem, currentSelection int) {
	screen.Clear()
	width, height := screen.Size()
	separatorX := width / 5
	previewStartX := separatorX + 3

	// Draw vertical line separator
	for y := 0; y < height-2; y++ {
		screen.SetContent(separatorX, y, '│', nil, tcell.StyleDefault)
	}

	// Render the tree and highlight the current selection
	for i, item := range tree {
		line := formatTreeItem(item)
		style := tcell.StyleDefault
		if i == currentSelection {
			style = style.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite)
			renderMarkdownPreview(item.Path, previewStartX)
		}
		renderText(0, i, line, style)
	}

	// Draw horizontal separator above the footer
	renderHorizontalSeparator(0, height-2, width)

	// Render the footer with available key actions
	renderFooter(tree[currentSelection])
	screen.Show()
}

func renderHorizontalSeparator(x, y, width int) {
	for i := x; i < width; i++ {
		screen.SetContent(i, y, '─', nil, tcell.StyleDefault)
	}
}

// Helper function to clear a rectangular area on the screen
func renderClearArea(x1, y1, x2, y2 int) {
	for x := x1; x < x2; x++ {
		for y := y1; y < y2; y++ {
			screen.SetContent(x, y, ' ', nil, tcell.StyleDefault)
		}
	}
}

// Show error messages
func renderError(message string) {
	width, height := screen.Size()
	// Display the message at the bottom of the screen
	y := height - 1
	renderClearArea(0, y, width, height)
	renderText(0, y, "Error: "+message+" (Press any key to continue)", tcell.StyleDefault.Foreground(tcell.ColorRed))
	screen.Show()
	// Wait for a key press to continue
	screen.PollEvent()
}

func renderInitScreen() {
	var errInit error
	screen, errInit = tcell.NewScreen()
	if errInit != nil {
		panic(errInit)
	}
	errInit = screen.Init()
	if errInit != nil {
		panic(errInit)
	}
}
