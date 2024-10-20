package main

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Handle creating a new file or directory
func handleNew(item TreeItem, rootItemPath string) {
	if !isDir(item.Path) {
		// Cannot create a new file or directory inside a file
		renderError("Cannot create new file or directory inside a file")
		return
	}

	// Get the path of the selected directory relative to the root
	currentRelPath, err := filepath.Rel(rootItemPath, item.Path)
	if err != nil {
		renderError("Error calculating relative path")
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
	name, ok := getUserInput(prompt, defaultInput)
	if !ok || name == "" {
		return
	}

	// Resolve and validate the path
	newPath, err := resolveAndValidatePath(name, rootItemPath)
	if err != nil {
		renderError(err.Error())
		return
	}

	// Determine if creating a directory or file based on the input
	if strings.HasSuffix(name, "/") {
		// Create a directory
		err := os.MkdirAll(newPath, os.ModePerm)
		if err != nil {
			renderError("Error creating directory: " + err.Error())
			return
		}
	} else {
		// Ensure the directory exists
		dirPath := filepath.Dir(newPath)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			err = os.MkdirAll(dirPath, os.ModePerm)
			if err != nil {
				renderError("Error creating directories: " + err.Error())
				return
			}
		}
		// Create the file
		file, err := os.Create(newPath)
		if err != nil {
			renderError("Error creating file: " + err.Error())
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
			renderError("Error deleting: " + err.Error())
		}
	}
}

func handleMove(item TreeItem, rootItemPath string) {
	if !isFile(item.Path) {
		// Can't move a directory using 'M' key
		renderError("Move operation is only for files")
		return
	}

	// Calculate the path relative to the notes directory
	currentRelPath, err := filepath.Rel(rootItemPath, item.Path)
	if err != nil {
		renderError("Error calculating relative path")
		return
	}

	prompt := "Enter new path (relative to notes directory): "
	inputPath, ok := getUserInput(prompt, currentRelPath)
	if !ok || inputPath == "" || inputPath == currentRelPath {
		return
	}

	// Resolve the new path relative to the notes directory
	newPath, err := resolveAndValidatePath(inputPath, rootItemPath)
	if err != nil {
		renderError(err.Error())
		return
	}

	// Check if the destination file already exists
	if _, err := os.Stat(newPath); err == nil {
		renderError("File already exists at destination")
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
			renderError("Error creating directories: " + err.Error())
			return
		}
	}

	// Move the file
	err = os.Rename(item.Path, newPath)
	if err != nil {
		renderError("Error moving file: " + err.Error())
		return
	}
}

// Handle renaming of directories
func handleRename(item TreeItem) {
	currentName := filepath.Base(item.Path)
	prompt := "Enter new name: "
	newName, ok := getUserInput(prompt, currentName)
	if !ok || newName == "" || newName == currentName {
		return
	}

	newPath := filepath.Join(filepath.Dir(item.Path), newName)

	// Check if the destination already exists
	if _, err := os.Stat(newPath); err == nil {
		confirmPrompt := "A file or directory with that name already exists. Overwrite? (y/N): "
		if !getConfirmation(confirmPrompt) {
			return
		}
	}

	err := os.Rename(item.Path, newPath)
	if err != nil {
		renderError("Error renaming: " + err.Error())
	}
}

// Get user input for prompts
func getUserInput(prompt string, defaultValue string) (string, bool) {
	input := []rune(defaultValue)
	cursorPos := len(input)
	width, height := screen.Size()
	promptY := height - 1

	for {
		// Clear the line
		renderClearArea(0, promptY, width, height)
		// Render the prompt and input
		renderText(0, promptY, prompt+string(input), tcell.StyleDefault)
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
func getConfirmation(prompt string) bool {
	width, height := screen.Size()
	promptY := height - 1
	for {
		// Clear the line
		renderClearArea(0, promptY, width, height)
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

// Open a file in Vim
func openVim(path string) {
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
		renderInitScreen()
		renderError("Error opening Vim: " + err.Error())
		return
	}

	// Re-create and re-initialize the screen after Vim exits
	renderInitScreen()
}
