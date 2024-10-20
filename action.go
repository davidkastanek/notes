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
func handleNew(item TreeItem) {
	if !isDir(item.Path) {
		// Cannot create new file inside a file
		renderError("Cannot create new file inside a file")
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
			renderError("Error creating directory: " + err.Error())
			return
		}
	} else {
		// Create directories if needed
		dirPath := filepath.Dir(newPath)
		err := os.MkdirAll(dirPath, os.ModePerm)
		if err != nil {
			renderError("Error creating directories: " + err.Error())
			return
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

// Handle moving a file
func handleMove(item TreeItem) {
	if !isFile(item.Path) {
		// Can't move a directory using 'M' key
		renderError("Move operation is only for files")
		return
	}

	prompt := "Enter new path: "
	newPath, ok := getUserInput(prompt)
	if !ok || newPath == "" {
		return
	}

	newPath = prefixRelativePath(newPath)

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
	err := os.Rename(item.Path, newPath)
	if err != nil {
		renderError("Error moving file: " + err.Error())
		return
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
		renderError("Error renaming: " + err.Error())
	}
}

// Get user input for prompts
func getUserInput(prompt string) (string, bool) {
	input := ""
	width, height := screen.Size()
	promptY := height - 1

	// Clear the line
	renderClearArea(0, promptY, width, height)
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
		renderClearArea(0, promptY, width, height)
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
