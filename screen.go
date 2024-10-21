package main

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"os"
	"os/exec"
)

func initScreen() (tcell.Screen, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, fmt.Errorf("error creating screen: %v", err)
	}
	err = screen.Init()
	if err != nil {
		return nil, fmt.Errorf("error initializing screen: %v", err)
	}
	return screen, nil
}

func resetScreen(screen tcell.Screen) {
	screen.Fini()
	cmd := exec.Command("stty", "sane")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func renderError(message string, screen tcell.Screen) {
	width, height := screen.Size()
	y := height - 1 // Display the message at the bottom of the screen
	renderClearArea(0, y, width, height, screen)
	renderText(0, y, "Error: "+message+" (Press any key to continue)", tcell.StyleDefault.Foreground(tcell.ColorRed), screen)
	screen.Show()
	screen.PollEvent() // Wait for a key press to continue
}

// Helper function to clear a rectangular area on the screen
func renderClearArea(x1, y1, x2, y2 int, screen tcell.Screen) {
	for x := x1; x < x2; x++ {
		for y := y1; y < y2; y++ {
			screen.SetContent(x, y, ' ', nil, tcell.StyleDefault)
		}
	}
}

func renderHorizontalSeparator(x, y, width int, screen tcell.Screen) {
	for i := x; i < width; i++ {
		screen.SetContent(i, y, '─', nil, tcell.StyleDefault)
	}
}

func renderFooter(selectedItem TreeItem, screen tcell.Screen) {
	width, height := screen.Size()
	hint := "M: Move | R: Rename | D: Delete | Q: Quit"
	if isDir(selectedItem.Path) {
		hint = "N: New | " + hint
	} else {
		hint = "E: Edit | " + hint
	}
	renderClearArea(0, height-1, width, height, screen)
	renderText(0, height-1, hint, tcell.StyleDefault, screen)
}

// Render a line of text on the screen at a given x, y position
func renderText(x, y int, text string, style tcell.Style, screen tcell.Screen) {
	col := x
	for _, r := range text {
		screen.SetContent(col, y, r, nil, style)
		col += runewidth.RuneWidth(r)
	}
}
