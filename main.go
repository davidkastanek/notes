package main

import (
	"bufio"
	"bytes"
	"errors"
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

func main() {
	d := flag.String("d", "", "Path to directory with notes")
	flag.Parse()
	if *d == "" {
		fmt.Println("Error: no directory provided. Use -d to specify a directory.")
		os.Exit(1)
	}
	dir := *d

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	screen, err := initScreen()
	if err != nil {
		exitWithError(err)
	}
	defer func() {
		resetScreen(screen)
	}()

	go func() {
		<-sigChan
		resetScreen(screen)
		os.Exit(0)
	}()

	if !isDir(dir) {
		exitWithError(errors.New("error: not a directory"))
	}

	rootItem := buildTree(dir)
	flatTree := flattenTree(rootItem, []bool{})
	var currentSelection = new(int)
	*currentSelection = 0

	for {
		renderTree(flatTree, currentSelection, screen)
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
					return
				case 'E', 'e':
					if isFile(flatTree[*currentSelection].Path) {
						screen, err = openVim(flatTree[*currentSelection].Path, screen)
						if err != nil {
							exitWithError(err)
						}
						flatTree = rebuildTree(dir, currentSelection)
					}
				case 'R', 'r':
					err = handleRename(flatTree[*currentSelection], screen)
					if err != nil {
						handleError(err, screen)
					}
					flatTree = rebuildTree(dir, currentSelection)
				case 'N', 'n':
					if isDir(flatTree[*currentSelection].Path) {
						err = handleNew(flatTree[*currentSelection], rootItem.Path, screen)
						if err != nil {
							handleError(err, screen)
						}
						flatTree = rebuildTree(dir, currentSelection)
					}
				case 'D', 'd':
					err = handleDelete(flatTree[*currentSelection], rootItem.Path, screen)
					if err != nil {
						handleError(err, screen)
					}
					flatTree = rebuildTree(dir, currentSelection)
				case 'M', 'm':
					err = handleMove(flatTree[*currentSelection], rootItem.Path, screen)
					if err != nil {
						handleError(err, screen)
					}
					flatTree = rebuildTree(dir, currentSelection)
				}
			}
		}
	}
}

type TreeItem struct {
	Display  string
	Path     string
	Children []TreeItem
	IsLast   bool
	Prefixes []bool
}

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

func rebuildTree(dir string, currentSelection *int) []TreeItem {
	rootItem := buildTree(dir)
	flatTree := flattenTree(rootItem, []bool{})
	if *currentSelection >= len(flatTree) {
		*currentSelection = len(flatTree) - 1
	}
	return flatTree
}

func buildTree(path string) TreeItem {
	rootItem := TreeItem{
		Display: filepath.Base(path),
		Path:    path,
		IsLast:  true,
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

	item.Prefixes = make([]bool, len(prefixes))
	copy(item.Prefixes, prefixes)

	flatTree = append(flatTree, item)

	numChildren := len(item.Children)
	for i, child := range item.Children {
		isLastChild := i == numChildren-1

		childPrefixes := append(prefixes, !isLastChild)

		flatTree = append(flatTree, flattenTree(child, childPrefixes)...)
	}

	return flatTree
}

func isFile(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !fileInfo.IsDir()
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

func resolveAndValidatePath(inputPath string, rootItemPath string) (string, error) {
	if strings.HasPrefix(inputPath, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("unable to determine home directory: %v", err)
		}
		inputPath = filepath.Join(homeDir, strings.TrimPrefix(inputPath, "~"))
	}

	resolvedPath := filepath.Clean(filepath.Join(rootItemPath, inputPath))

	relPath, err := filepath.Rel(rootItemPath, resolvedPath)
	if err != nil {
		return "", fmt.Errorf("error calculating relative path of %s against basepath %s", resolvedPath, rootItemPath)
	}

	if strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) || relPath == ".." {
		return "", userErr{"Path must be within the notes directory"}
	}

	return resolvedPath, nil
}

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

func parseANSICode(code string, style TextStyle) TextStyle {
	parts := strings.Split(code, ";")
	for _, part := range parts {
		switch part {
		case "0":
			style = TextStyle{}
		case "1":
			style.Bold = true
		case "4":
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
		default:
		}
	}
	return style
}

func processANSIStrings(s string) []ColData {
	var cols []ColData
	var currentStyle TextStyle
	var textBuilder strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+2 < len(s) && s[i+1] == '[' {
			if textBuilder.Len() > 0 {
				cols = append(cols, ColData{
					Text:  textBuilder.String(),
					Style: currentStyle,
				})
				textBuilder.Reset()
			}
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

func getUserInput(prompt string, defaultValue string, screen tcell.Screen) (string, bool) {
	input := []rune(defaultValue)
	cursorPos := len(input)
	width, height := screen.Size()
	promptY := height - 1

	for {
		renderClearArea(0, promptY, width, height, screen)
		renderText(0, promptY, prompt+string(input), tcell.StyleDefault, screen)
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

func getConfirmation(prompt string, screen tcell.Screen) bool {
	width, height := screen.Size()
	promptY := height - 1
	for {
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

func openVim(path string, screen tcell.Screen) (tcell.Screen, error) {
	resetScreen(screen)

	cmd := exec.Command("vim", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error opening vim at %s: %w", path, err)
	}

	screen, err = initScreen()
	if err != nil {
		return nil, fmt.Errorf("error initializing screen after vim close: %v", err)
	}

	return screen, nil
}

func renderMarkdownPreview(path string, startX int, screen tcell.Screen) {
	width, height := screen.Size()
	if isFile(path) {
		source, err := os.ReadFile(path)
		if err != nil {
			return
		}
		lines := markdown.Render(string(source), (width-width/5)-2, 0)
		renderClearArea(startX, 0, width, height-2, screen)
		renderMarkdown(startX, 1, lines, screen)
	} else {
		renderClearArea(startX, 0, width, height-2, screen)
	}
}

func renderTree(tree []TreeItem, currentSelection *int, screen tcell.Screen) {
	screen.Clear()
	width, height := screen.Size()
	separatorX := width / 5
	previewStartX := separatorX + 3

	for y := 0; y < height-2; y++ {
		screen.SetContent(separatorX, y, '│', nil, tcell.StyleDefault)
	}

	for i, item := range tree {
		line := formatTreeItem(item)
		style := tcell.StyleDefault
		if i == *currentSelection {
			style = style.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite)
			renderMarkdownPreview(item.Path, previewStartX, screen)
		}
		renderText(0, i, line, style, screen)
	}

	renderHorizontalSeparator(0, height-2, width, screen)

	renderFooter(tree[*currentSelection], screen)
	screen.Show()
}
