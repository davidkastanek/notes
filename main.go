package main

import (
	"flag"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

var screen tcell.Screen
var currentSelection int

type TreeItem struct {
	Display  string     // The string to display
	Path     string     // The full path to the item
	Children []TreeItem // Child items (for directories)
	IsLast   bool       // Whether this item is the last child at its level
	Prefixes []bool     // Indentation prefixes
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

	rootItem := buildTree(dir)
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
					if isFile(flatTree[currentSelection].Path) {
						openVim(flatTree[currentSelection].Path)
					}
					// Rebuild the tree after selection
					rootItem := buildTree(dir)
					flatTree = flattenTree(rootItem, []bool{})
					if currentSelection >= len(flatTree) {
						currentSelection = len(flatTree) - 1
					}
				case 'R', 'r':
					handleRename(flatTree[currentSelection])
					// Rebuild the tree after selection
					rootItem := buildTree(dir)
					flatTree = flattenTree(rootItem, []bool{})
					if currentSelection >= len(flatTree) {
						currentSelection = len(flatTree) - 1
					}
				case 'N', 'n':
					if isDir(flatTree[currentSelection].Path) {
						handleNew(flatTree[currentSelection], rootItem.Path)
						rootItem := buildTree(dir)
						flatTree = flattenTree(rootItem, []bool{})
						if currentSelection >= len(flatTree) {
							currentSelection = len(flatTree) - 1
						}
					}
				case 'D', 'd':
					handleDelete(flatTree[currentSelection])
					rootItem := buildTree(dir)
					flatTree = flattenTree(rootItem, []bool{})
					if currentSelection >= len(flatTree) {
						currentSelection = len(flatTree) - 1
					}
				case 'M', 'm':
					if isFile(flatTree[currentSelection].Path) {
						handleMove(flatTree[currentSelection], rootItem.Path)
						rootItem := buildTree(dir)
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
	// Expand '~' to user home directory
	if strings.HasPrefix(inputPath, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("Unable to determine home directory")
		}
		inputPath = filepath.Join(homeDir, strings.TrimPrefix(inputPath, "~"))
	}

	// If the path is not absolute, make it relative to the notes directory
	var resolvedPath string
	if !filepath.IsAbs(inputPath) {
		resolvedPath = filepath.Join(rootItemPath, inputPath)
	} else {
		resolvedPath = inputPath
	}

	// Clean the path (resolve '..', '.', etc.)
	resolvedPath = filepath.Clean(resolvedPath)

	// Get absolute paths for comparison
	absResolvedPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("Invalid path")
	}
	absRoot, err := filepath.Abs(rootItemPath)
	if err != nil {
		return "", fmt.Errorf("Invalid root path")
	}

	// Check if the resolved path is within the notes directory
	if !strings.HasPrefix(absResolvedPath, absRoot) {
		return "", fmt.Errorf("Path must be within the notes directory")
	}

	return resolvedPath, nil
}
