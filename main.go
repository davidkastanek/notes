package main

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

var screen tcell.Screen
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
