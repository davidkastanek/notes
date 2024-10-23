package main

import (
	"errors"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"os"
	"path/filepath"
	"strings"
)

func handleRename(item TreeItem, screen tcell.Screen) error {
	currentName := filepath.Base(item.Path)
	prompt := "Enter new name: "
	newName, ok := getUserInput(prompt, currentName, screen)
	if !ok || newName == "" || newName == currentName {
		return nil
	}

	newPath := filepath.Join(filepath.Dir(item.Path), newName)
	if _, err := os.Stat(newPath); err == nil {
		confirmPrompt := "A file or directory with that name already exists. Overwrite? (y/N): "
		if !getConfirmation(confirmPrompt, screen) {
			return nil
		}
	}

	err := os.Rename(item.Path, newPath)
	if err != nil {
		return fmt.Errorf("error renaming directory %s to %s: %v", item.Path, newPath, err)
	}

	return nil
}

func handleMove(item TreeItem, rootItemPath string, screen tcell.Screen) error {
	if item.Path == rootItemPath {
		return userErr{"Cannot move to root directory"}
	}

	currentRelPath, err := filepath.Rel(rootItemPath, item.Path)
	if err != nil {
		return fmt.Errorf("error calculating relative path of %s against basepath %s", item.Path, rootItemPath)
	}

	prompt := "Enter new path: "
	inputPath, ok := getUserInput(prompt, currentRelPath, screen)
	if !ok || inputPath == "" || inputPath == currentRelPath {
		return nil
	}

	newPath, err := resolveAndValidatePath(inputPath, rootItemPath)
	if err != nil {
		var userErr *userErr
		if errors.As(err, &userErr) {
			return err
		}
		return fmt.Errorf("error resolving & validating path %s against %s: %v", inputPath, rootItemPath, err)
	}

	itemAbsPath, err := filepath.Abs(item.Path)
	if err != nil {
		return fmt.Errorf("error getting absolute path for %s: %v", item.Path, err)
	}
	newAbsPath, err := filepath.Abs(newPath)
	if err != nil {
		return fmt.Errorf("error getting absolute path for %s: %v", item.Path, err)
	}
	if strings.HasPrefix(newAbsPath, itemAbsPath+string(os.PathSeparator)) {
		return userErr{"Cannot move a directory into itself or its subdirectory"}
	}

	if _, err := os.Stat(newPath); err == nil {
		confirmPrompt := "Destination exists. Overwrite? (y/N): "
		if !getConfirmation(confirmPrompt, screen) {
			return nil
		}
	}

	dir := filepath.Dir(newPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		confirmPrompt := "Directory does not exist. Create parent directories and move? (y/N): "
		if !getConfirmation(confirmPrompt, screen) {
			return nil
		}
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return fmt.Errorf("error creating parent directory %s: %v", dir, err)
		}
	}

	err = os.Rename(item.Path, newPath)
	if err != nil {
		return fmt.Errorf("error moving directory %s to %s: %v", item.Path, newAbsPath, err)
	}

	return nil
}

func handleDelete(item TreeItem, rootItemPath string, screen tcell.Screen) error {
	if item.Path == rootItemPath {
		return userErr{"Cannot delete the root directory"}
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
			return fmt.Errorf("error deleting file: %v", err)
		}
	}
	return nil
}

func handleNew(item TreeItem, rootItemPath string, screen tcell.Screen) error {
	if !isDir(item.Path) {
		return fmt.Errorf("cannot create new file or directory inside a file: %s", item.Path)
	}

	currentRelPath, err := filepath.Rel(rootItemPath, item.Path)
	if err != nil {
		return fmt.Errorf("error calculating relative path of %s against basepath %s", item.Path, rootItemPath)
	}

	var defaultInput string
	if currentRelPath == "." {
		defaultInput = ""
	} else {
		defaultInput = currentRelPath + "/"
	}

	prompt := "Enter new name: "
	name, ok := getUserInput(prompt, defaultInput, screen)
	if !ok || name == "" {
		return nil
	}

	newPath, err := resolveAndValidatePath(name, rootItemPath)
	if err != nil {
		var userErr *userErr
		if errors.As(err, &userErr) {
			return err
		}
		return fmt.Errorf("error resolving & validating path %s against %s: %v", name, rootItemPath, err)
	}

	if strings.HasSuffix(name, "/") {
		err := os.MkdirAll(newPath, os.ModePerm)
		if err != nil {
			return fmt.Errorf("error creating directory %s: %v", newPath, err)
		}
	} else {
		dirPath := filepath.Dir(newPath)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			err = os.MkdirAll(dirPath, os.ModePerm)
			if err != nil {
				return fmt.Errorf("error creating directory %s: %v", dirPath, err)
			}
		}

		file, err := os.Create(newPath)
		if err != nil {
			return fmt.Errorf("error creating file %s: %v", newPath, err)
		}
		err = file.Close()
		if err != nil {
			return fmt.Errorf("error closing file %s: %v", newPath, err)
		}
	}

	return nil
}
