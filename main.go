package main

import (
	"bufio"
	"bytes"
	"flag"
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

type TreeItem struct {
	Display  string     // The string to display
	Path     string     // The full path to the item
	Children []TreeItem // Child items (for directories)
	IsLast   bool       // Whether this item is the last child at its level
	Prefixes []bool     // Indentation prefixes
}

// ColData Struct to hold text and style after processing ANSI escape sequences
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

func main() {
	d := flag.String("d", "", "Path to directory with notes")
	flag.Parse()
	if *d == "" {
		fmt.Println("Error: no directory provided. Use -d to specify a directory.")
		os.Exit(1)
	}
	dir := *d

	// Create a channel to listen for termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var err error
	// Initialize the terminal screen
	var screen tcell.Screen
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
	}()

	// Start a goroutine to listen for signals
	go func() {
		<-sigChan
		screen.Fini()
		fmt.Print("\033[H\033[2J")
		os.Exit(0)
	}()

	if !isDir(dir) {
		panic("cannot open root directory" + dir)
	}

	rootItem := buildTree(dir)
	flatTree := flattenTree(rootItem, []bool{})
	var currentSelection = new(int)
	*currentSelection = 0

	// Main loop for rendering and interacting with tree
	for {
		renderTree(flatTree, currentSelection, screen)
		// Capture input and handle actions (navigation and commands)
		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyUp:
				if *currentSelection > 0 {
					*currentSelection--
				}
			case tcell.KeyDown:
				if *currentSelection < len(flatTree)-1 {
					*currentSelection++
				}
			case tcell.KeyEscape, tcell.KeyCtrlC:
				return
			case tcell.KeyRune:
				switch ev.Rune() {
				case 'Q', 'q':
					// Exit the application
					return
				case 'E', 'e':
					if isFile(flatTree[*currentSelection].Path) {
						screen = openVim(flatTree[*currentSelection].Path, screen)
					}
					flatTree = rebuildTree(dir, currentSelection)
				case 'R', 'r':
					handleRename(flatTree[*currentSelection], screen)
					flatTree = rebuildTree(dir, currentSelection)
				case 'N', 'n':
					if isDir(flatTree[*currentSelection].Path) {
						handleNew(flatTree[*currentSelection], rootItem.Path, screen)
						flatTree = rebuildTree(dir, currentSelection)
					}
				case 'D', 'd':
					handleDelete(flatTree[*currentSelection], rootItem.Path, screen)
					flatTree = rebuildTree(dir, currentSelection)
				case 'M', 'm':
					handleMove(flatTree[*currentSelection], rootItem.Path, screen)
					flatTree = rebuildTree(dir, currentSelection)
				}
			}
		}
	}
}

func rebuildTree(dir string, currentSelection *int) []TreeItem {
	rootItem := buildTree(dir)
	flatTree := flattenTree(rootItem, []bool{})
	if *currentSelection >= len(flatTree) {
		*currentSelection = len(flatTree) - 1
	}
	return flatTree
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

func resolveAndValidatePath(inputPath string, rootItemPath string) (string, error) {
	// Expand '~' to user home directory if present
	if strings.HasPrefix(inputPath, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("Unable to determine home directory")
		}
		inputPath = filepath.Join(homeDir, strings.TrimPrefix(inputPath, "~"))
	}

	// Combine the root path and input path, then clean the path
	resolvedPath := filepath.Clean(filepath.Join(rootItemPath, inputPath))

	// Ensure the resolved path is within the root directory
	relPath, err := filepath.Rel(rootItemPath, resolvedPath)
	if err != nil {
		return "", fmt.Errorf("Invalid path")
	}

	// If the relative path starts with '..', it's outside the root directory
	if strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) || relPath == ".." {
		return "", fmt.Errorf("Path must be within the notes directory")
	}

	return resolvedPath, nil
}

// Helper function to render markdown output including ANSI escape sequences
func renderMarkdown(x, y int, content []byte, screen tcell.Screen) {
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

func formatTreeItem(item TreeItem) string {
	var builder strings.Builder

	for i := 0; i < len(item.Prefixes)-1; i++ {
		if item.Prefixes[i] {
			builder.WriteString("│  ")
		} else {
			builder.WriteString("   ")
		}
	}

	if len(item.Prefixes) > 0 {
		if item.Prefixes[len(item.Prefixes)-1] {
			builder.WriteString("├─ ")
		} else {
			builder.WriteString("└─ ")
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

// Handle creating a new file or directory
func handleNew(item TreeItem, rootItemPath string, screen tcell.Screen) {
	if !isDir(item.Path) {
		// Cannot create a new file or directory inside a file
		renderError("Cannot create new file or directory inside a file", screen)
		return
	}

	// Get the path of the selected directory relative to the root
	currentRelPath, err := filepath.Rel(rootItemPath, item.Path)
	if err != nil {
		renderError("Error calculating relative path", screen)
		return
	}

	// Prepopulate default input with current relative path
	var defaultInput string
	if currentRelPath == "." {
		defaultInput = ""
	} else {
		defaultInput = currentRelPath + "/"
	}

	prompt := "Enter new name: "
	name, ok := getUserInput(prompt, defaultInput, screen)
	if !ok || name == "" {
		return
	}

	// Resolve and validate the path
	newPath, err := resolveAndValidatePath(name, rootItemPath)
	if err != nil {
		renderError(err.Error(), screen)
		return
	}

	// Determine if creating a directory or file based on the input
	if strings.HasSuffix(name, "/") {
		// Create a directory
		err := os.MkdirAll(newPath, os.ModePerm)
		if err != nil {
			renderError("Error creating directory: "+err.Error(), screen)
			return
		}
	} else {
		// Ensure the directory exists
		dirPath := filepath.Dir(newPath)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			err = os.MkdirAll(dirPath, os.ModePerm)
			if err != nil {
				renderError("Error creating directories: "+err.Error(), screen)
				return
			}
		}
		// Create the file
		file, err := os.Create(newPath)
		if err != nil {
			renderError("Error creating file: "+err.Error(), screen)
			return
		}
		file.Close()
	}
}

// Handle deleting files or directories
func handleDelete(item TreeItem, rootItemPath string, screen tcell.Screen) {
	if item.Path == rootItemPath {
		renderError("Cannot delete the root directory", screen)
		return
	}
	prompt := "Are you sure you want to delete " + item.Path + "? (y/N): "
	if getConfirmation(prompt, screen) {
		var err error
		if isDir(item.Path) {
			err = os.RemoveAll(item.Path)
		} else {
			err = os.Remove(item.Path)
		}
		if err != nil {
			renderError("Error deleting: "+err.Error(), screen)
		}
	}
}

func handleMove(item TreeItem, rootItemPath string, screen tcell.Screen) {
	if item.Path == rootItemPath {
		renderError("Cannot move the root directory", screen)
		return
	}

	// Calculate the path relative to the notes directory
	currentRelPath, err := filepath.Rel(rootItemPath, item.Path)
	if err != nil {
		renderError("Error calculating relative path", screen)
		return
	}

	prompt := "Enter new path: "
	inputPath, ok := getUserInput(prompt, currentRelPath, screen)
	if !ok || inputPath == "" || inputPath == currentRelPath {
		return
	}

	// Use the updated resolveAndValidatePath function
	newPath, err := resolveAndValidatePath(inputPath, rootItemPath)
	if err != nil {
		renderError(err.Error(), screen)
		return
	}

	// Check if moving a directory into itself or its subdirectory
	itemAbsPath, err := filepath.Abs(item.Path)
	if err != nil {
		renderError("Invalid source path", screen)
		return
	}
	newAbsPath, err := filepath.Abs(newPath)
	if err != nil {
		renderError("Invalid destination path", screen)
		return
	}
	if strings.HasPrefix(newAbsPath, itemAbsPath+string(os.PathSeparator)) {
		renderError("Cannot move a directory into itself or its subdirectory", screen)
		return
	}

	// Check if the destination already exists
	if _, err := os.Stat(newPath); err == nil {
		// Destination exists
		confirmPrompt := "Destination exists. Overwrite? (y/N): "
		if !getConfirmation(confirmPrompt, screen) {
			return
		}
	}

	// For moving, ensure the parent directory of the destination exists
	dir := filepath.Dir(newPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		confirmPrompt := "Directory does not exist. Create parent directories and move? (y/N): "
		if !getConfirmation(confirmPrompt, screen) {
			return
		}
		// Create required directories
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			renderError("Error creating directories: "+err.Error(), screen)
			return
		}
	}

	// Move the file or directory
	err = os.Rename(item.Path, newPath)
	if err != nil {
		renderError("Error moving: "+err.Error(), screen)
		return
	}
}

// Handle renaming of directories
func handleRename(item TreeItem, screen tcell.Screen) {
	currentName := filepath.Base(item.Path)
	prompt := "Enter new name: "
	newName, ok := getUserInput(prompt, currentName, screen)
	if !ok || newName == "" || newName == currentName {
		return
	}

	newPath := filepath.Join(filepath.Dir(item.Path), newName)

	// Check if the destination already exists
	if _, err := os.Stat(newPath); err == nil {
		confirmPrompt := "A file or directory with that name already exists. Overwrite? (y/N): "
		if !getConfirmation(confirmPrompt, screen) {
			return
		}
	}

	err := os.Rename(item.Path, newPath)
	if err != nil {
		renderError("Error renaming: "+err.Error(), screen)
	}
}

// Get user input for prompts
func getUserInput(prompt string, defaultValue string, screen tcell.Screen) (string, bool) {
	input := []rune(defaultValue)
	cursorPos := len(input)
	width, height := screen.Size()
	promptY := height - 1

	for {
		// Clear the line
		renderClearArea(0, promptY, width, height, screen)
		// Render the prompt and input
		renderText(0, promptY, prompt+string(input), tcell.StyleDefault, screen)
		// Move the cursor to the correct position
		screen.ShowCursor(len(prompt)+cursorPos, promptY)
		screen.Show()

		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEsc, tcell.KeyCtrlC:
				screen.HideCursor()
				return "", false
			case tcell.KeyEnter:
				screen.HideCursor()
				return string(input), true
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if cursorPos > 0 {
					input = append(input[:cursorPos-1], input[cursorPos:]...)
					cursorPos--
				}
			case tcell.KeyDelete:
				if cursorPos < len(input) {
					input = append(input[:cursorPos], input[cursorPos+1:]...)
				}
			case tcell.KeyLeft:
				if cursorPos > 0 {
					cursorPos--
				}
			case tcell.KeyRight:
				if cursorPos < len(input) {
					cursorPos++
				}
			case tcell.KeyHome:
				cursorPos = 0
			case tcell.KeyEnd:
				cursorPos = len(input)
			default:
				if ev.Rune() != 0 {
					input = append(input[:cursorPos], append([]rune{ev.Rune()}, input[cursorPos:]...)...)
					cursorPos++
				}
			}
		}
	}
}

// Get confirmation (yes/no) from user
func getConfirmation(prompt string, screen tcell.Screen) bool {
	width, height := screen.Size()
	promptY := height - 1
	for {
		// Clear the line
		renderClearArea(0, promptY, width, height, screen)
		renderText(0, promptY, prompt, tcell.StyleDefault, screen)
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

// Open a file in Vim
func openVim(path string, screen tcell.Screen) tcell.Screen {
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
		panic(err)
	}

	// Re-create and re-initialize the screen after Vim exits
	afterVimScreen, newScreenErr := renderInitScreen()
	if newScreenErr != nil {
		panic(newScreenErr)
	}

	return afterVimScreen
}

// Render a line of text on the screen at a given x, y position
func renderText(x, y int, text string, style tcell.Style, screen tcell.Screen) {
	col := x
	for _, r := range text {
		screen.SetContent(col, y, r, nil, style)
		col += runewidth.RuneWidth(r)
	}
}

// Render footer with key hints
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

// Render markdown preview for files
func renderMarkdownPreview(path string, startX int, screen tcell.Screen) {
	width, height := screen.Size()
	if isFile(path) {
		source, err := os.ReadFile(path)
		if err != nil {
			return
		}
		lines := markdown.Render(string(source), (width-width/5)-2, 0)
		// Clear previous preview content
		renderClearArea(startX, 0, width, height-2, screen)
		renderMarkdown(startX, 1, lines, screen)
	} else {
		// Clear preview area if not a file
		renderClearArea(startX, 0, width, height-2, screen)
	}
}

// Render the directory tree and highlight the current selection
func renderTree(tree []TreeItem, currentSelection *int, screen tcell.Screen) {
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
		if i == *currentSelection {
			style = style.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite)
			renderMarkdownPreview(item.Path, previewStartX, screen)
		}
		renderText(0, i, line, style, screen)
	}

	// Draw horizontal separator above the footer
	renderHorizontalSeparator(0, height-2, width, screen)

	// Render the footer with available key actions
	renderFooter(tree[*currentSelection], screen)
	screen.Show()
}

func renderHorizontalSeparator(x, y, width int, screen tcell.Screen) {
	for i := x; i < width; i++ {
		screen.SetContent(i, y, '─', nil, tcell.StyleDefault)
	}
}

// Helper function to clear a rectangular area on the screen
func renderClearArea(x1, y1, x2, y2 int, screen tcell.Screen) {
	for x := x1; x < x2; x++ {
		for y := y1; y < y2; y++ {
			screen.SetContent(x, y, ' ', nil, tcell.StyleDefault)
		}
	}
}

// Show error messages
func renderError(message string, screen tcell.Screen) {
	width, height := screen.Size()
	// Display the message at the bottom of the screen
	y := height - 1
	renderClearArea(0, y, width, height, screen)
	renderText(0, y, "Error: "+message+" (Press any key to continue)", tcell.StyleDefault.Foreground(tcell.ColorRed), screen)
	screen.Show()
	// Wait for a key press to continue
	screen.PollEvent()
}

func renderInitScreen() (tcell.Screen, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, fmt.Errorf("failed to create tcell screen: %v", err)
	}
	err = screen.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to init screen: %v", err)
	}
	return screen, nil
}
