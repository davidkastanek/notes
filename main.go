package main

import (
	"bufio"
	"bytes"
	"fmt"
	markdown "github.com/MichaelMure/go-term-markdown"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

var screen tcell.Screen
var tree []TreeItem
var currentSelection int
var root string

type TreeItem struct {
	Display  string     // The string to display
	Path     string     // The full path to the item
	Children []TreeItem // Child items (for directories)
	IsLast   bool       // Whether this item is the last child at its level
	Prefixes []bool     // Indentation prefixes
}

func main() {
	root = "root"
	var err error

	// Create a channel to listen for termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Initialize the terminal screen
	screen, err = tcell.NewScreen()
	if err != nil {
		panic(err)
	}
	err = screen.Init()
	if err != nil {
		panic(err)
	}
	defer func() {
		screen.Fini()
		resetTerminal()
		fmt.Print("\033[H\033[2J") // Clear the console window on exit
	}()

	// Start a goroutine to listen for signals
	go func() {
		<-sigChan
		screen.Fini()
		fmt.Print("\033[H\033[2J")
		os.Exit(0)
	}()

	rootItem := buildTree(root)
	flatTree := flattenTree(rootItem, []bool{})
	currentSelection = 0

	// Main loop for rendering and interacting with tree
	for {
		renderTree(flatTree, currentSelection)
		// Capture input and handle actions (navigation and commands)
		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyUp:
				if currentSelection > 0 {
					currentSelection--
				}
			case tcell.KeyDown:
				if currentSelection < len(flatTree)-1 {
					currentSelection++
				}
			case tcell.KeyEscape, tcell.KeyCtrlC:
				return
			case tcell.KeyRune:
				switch ev.Rune() {
				case 'Q', 'q':
					// Exit the application
					return
				case 'E', 'e':
					handleSelection(flatTree[currentSelection])
					// Rebuild the tree after selection
					rootItem := buildTree(root)
					flatTree = flattenTree(rootItem, []bool{})
					if currentSelection >= len(flatTree) {
						currentSelection = len(flatTree) - 1
					}
				case 'N', 'n':
					if isDir(flatTree[currentSelection].Path) {
						handleNew(flatTree[currentSelection])
						rootItem := buildTree(root)
						flatTree = flattenTree(rootItem, []bool{})
						if currentSelection >= len(flatTree) {
							currentSelection = len(flatTree) - 1
						}
					}
				case 'D', 'd':
					handleDelete(flatTree[currentSelection])
					rootItem := buildTree(root)
					flatTree = flattenTree(rootItem, []bool{})
					if currentSelection >= len(flatTree) {
						currentSelection = len(flatTree) - 1
					}
				case 'M', 'm':
					if isFile(flatTree[currentSelection].Path) {
						handleMove(flatTree[currentSelection])
						rootItem := buildTree(root)
						flatTree = flattenTree(rootItem, []bool{})
						if currentSelection >= len(flatTree) {
							currentSelection = len(flatTree) - 1
						}
					}
				}
			}
		}
	}
}

func resetTerminal() {
	cmd := exec.Command("stty", "sane")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

// Build the tree recursively and return the root TreeItem
func buildTree(path string) TreeItem {
	rootItem := TreeItem{
		Display: filepath.Base(path),
		Path:    path,
		IsLast:  true, // Root is considered the last item at its level
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return rootItem
	}

	numEntries := len(entries)
	for i, entry := range entries {
		itemPath := filepath.Join(path, entry.Name())
		isLastEntry := i == numEntries-1
		childItem := TreeItem{
			Display: entry.Name(),
			Path:    itemPath,
			IsLast:  isLastEntry,
		}

		if entry.IsDir() {
			childItem = buildTree(itemPath)
			childItem.Display = entry.Name()
			childItem.Path = itemPath
			childItem.IsLast = isLastEntry
		}

		rootItem.Children = append(rootItem.Children, childItem)
	}
	return rootItem
}

func flattenTree(item TreeItem, prefixes []bool) []TreeItem {
	var flatTree []TreeItem

	// Set prefixes for this item
	item.Prefixes = make([]bool, len(prefixes))
	copy(item.Prefixes, prefixes)

	flatTree = append(flatTree, item)

	numChildren := len(item.Children)
	for i, child := range item.Children {
		isLastChild := i == numChildren-1

		// Append to prefixes
		childPrefixes := append(prefixes, !isLastChild)

		flatTree = append(flatTree, flattenTree(child, childPrefixes)...)
	}

	return flatTree
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
	drawHorizontalSeparator(0, height-2, width)

	// Render the footer with available key actions
	renderFooter(tree[currentSelection])
	screen.Show()
}

func drawHorizontalSeparator(x, y, width int) {
	for i := x; i < width; i++ {
		screen.SetContent(i, y, '─', nil, tcell.StyleDefault)
	}
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
		clearArea(startX, 0, width, height-2)
		renderMarkdown(startX, 0, lines)
	} else {
		// Clear preview area if not a file
		clearArea(startX, 0, width, height-2)
	}
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

// Helper function to clear a rectangular area on the screen
func clearArea(x1, y1, x2, y2 int) {
	for x := x1; x < x2; x++ {
		for y := y1; y < y2; y++ {
			screen.SetContent(x, y, ' ', nil, tcell.StyleDefault)
		}
	}
}

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
		hint = "N: New Dir | F: New File | E: Rename | D: Delete | Q: Quit"
	}
	clearArea(0, height-1, width, height)
	renderText(0, height-1, hint, tcell.StyleDefault)
}

// Handle actions when an item is selected
func handleSelection(item TreeItem) {
	if isFile(item.Path) {
		openInVim(item.Path)
	} else {
		handleRename(item)
	}
}

// Open a file in Vim
func openInVim(path string) {
	// Finalize the screen and restore the terminal state
	screen.Fini()

	// Run Vim
	cmd := exec.Command("vim", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error opening file: %s\n", err)
		// Re-initialize the screen before returning
		initializeScreen()
		showError("Error opening Vim: " + err.Error())
		return
	}

	// Re-create and re-initialize the screen after Vim exits
	initializeScreen()
}

func initializeScreen() {
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

// Handle renaming of directories
func handleRename(item TreeItem) {
	prompt := "Enter new name: "
	newName, ok := getUserInput(prompt)
	if !ok || newName == "" {
		return
	}

	newPath := filepath.Join(filepath.Dir(item.Path), newName)

	err := os.Rename(item.Path, newPath)
	if err != nil {
		showError("Error renaming: " + err.Error())
	}
}

// Handle creating a new file or directory
func handleNew(item TreeItem) {
	if !isDir(item.Path) {
		// Cannot create new file inside a file
		showError("Cannot create new file inside a file")
		return
	}

	prompt := "Enter new file/directory name: "
	name, ok := getUserInput(prompt)
	if !ok || name == "" {
		return
	}

	newPath := filepath.Join(item.Path, name)

	if strings.HasSuffix(name, "/") {
		// Create a directory
		err := os.MkdirAll(newPath, os.ModePerm)
		if err != nil {
			showError("Error creating directory: " + err.Error())
			return
		}
	} else {
		// Create directories if needed
		dirPath := filepath.Dir(newPath)
		err := os.MkdirAll(dirPath, os.ModePerm)
		if err != nil {
			showError("Error creating directories: " + err.Error())
			return
		}

		// Create the file
		file, err := os.Create(newPath)
		if err != nil {
			showError("Error creating file: " + err.Error())
			return
		}
		file.Close()
	}
}

// Handle deleting files or directories
func handleDelete(item TreeItem) {
	prompt := "Are you sure you want to delete " + item.Path + "? (y/N): "
	if getConfirmation(prompt) {
		var err error
		if isDir(item.Path) {
			err = os.RemoveAll(item.Path)
		} else {
			err = os.Remove(item.Path)
		}
		if err != nil {
			showError("Error deleting: " + err.Error())
		}
	}
}

// Handle moving a file
func handleMove(item TreeItem) {
	if !isFile(item.Path) {
		// Can't move a directory using 'M' key
		showError("Move operation is only for files")
		return
	}

	prompt := "Enter new path: "
	newPath, ok := getUserInput(prompt)
	if !ok || newPath == "" {
		return
	}

	// Check if the destination file already exists
	if _, err := os.Stat(newPath); err == nil {
		showError("File already exists at destination")
		return
	}

	// Check if new directories need to be created
	dir := filepath.Dir(newPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		confirmPrompt := "Really move to " + newPath + "? (y/N): "
		if !getConfirmation(confirmPrompt) {
			return
		}
		// Create required directories
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			showError("Error creating directories: " + err.Error())
			return
		}
	}

	// Move the file
	err := os.Rename(item.Path, newPath)
	if err != nil {
		showError("Error moving file: " + err.Error())
		return
	}
}

// Get user input for prompts
func getUserInput(prompt string) (string, bool) {
	input := ""
	width, height := screen.Size()
	promptY := height - 1

	// Clear the line
	clearArea(0, promptY, width, height)
	renderText(0, promptY, prompt+input, tcell.StyleDefault)
	screen.Show()

	for {
		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEsc, tcell.KeyCtrlC:
				return "", false
			case tcell.KeyEnter:
				return input, true
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if len(input) > 0 {
					input = input[:len(input)-1]
				}
			default:
				if ev.Rune() != 0 {
					input += string(ev.Rune())
				}
			}
		}
		// Clear the line
		clearArea(0, promptY, width, height)
		renderText(0, promptY, prompt+input, tcell.StyleDefault)
		screen.Show()
	}
}

// Get confirmation (yes/no) from user
func getConfirmation(prompt string) bool {
	width, height := screen.Size()
	promptY := height - 1
	for {
		// Clear the line
		clearArea(0, promptY, width, height)
		renderText(0, promptY, prompt, tcell.StyleDefault)
		screen.Show()

		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Rune() {
			case 'y', 'Y':
				return true
			case 'n', 'N', 0:
				return false
			}
			if ev.Key() == tcell.KeyEnter {
				return false
			}
		}
	}
}

// Show error messages
func showError(message string) {
	width, height := screen.Size()
	// Display the message at the bottom of the screen
	y := height - 1
	clearArea(0, y, width, height)
	renderText(0, y, "Error: "+message+" (Press any key to continue)", tcell.StyleDefault.Foreground(tcell.ColorRed))
	screen.Show()
	// Wait for a key press to continue
	screen.PollEvent()
}

// Check if the path is a file
func isFile(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !fileInfo.IsDir()
}

// Check if the path is a directory
func isDir(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}
