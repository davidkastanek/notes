package main

import (
	"bufio"
	"fmt"
	"github.com/MichaelMure/go-term-markdown"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	args := os.Args

	if len(args) == 1 {
		showHelp()
		return
	}

	switch args[1] {
	case "show":
		handleShow(args)
	case "new":
		handleNew(args)
	case "edit":
		handleEdit(args)
	case "delete":
		handleDelete(args)
	case "move":
		handleMove(args)
	default:
		fmt.Printf("Unknown command: %s\n", args[1])
		showHelp()
	}
}

func handleShow(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: ./td show <path-to-file>")
		return
	}

	path := args[2]
	if isReadableFile(path) {
		source, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		result := markdown.Render(string(source), 150, 0)
		fmt.Println(string(result))
	} else {
		fmt.Printf("Error: %s is not a readable file or does not exist\n", path)
	}
}

func handleNew(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: ./td new <path-to-file>")
		return
	}

	path := args[2]
	if isReadableFile(path) {
		fmt.Printf("Error: file %s already exists\n", path)
		return
	}

	err := os.MkdirAll(filepath.Dir(path), os.ModePerm)
	if err != nil {
		fmt.Printf("Error creating directories: %s\n", err)
		return
	}

	cmd := exec.Command("vim", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Error opening vim: %s\n", err)
	}
}

func handleEdit(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: ./td edit <path-to-file>")
		return
	}

	path := args[2]
	if isReadableFile(path) {
		cmd := exec.Command("vim", path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			fmt.Printf("Error opening vim: %s\n", err)
		}
	} else {
		fmt.Printf("Error: %s does not exist\n", path)
	}
}

func handleDelete(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: ./td delete <path-to-file>")
		return
	}

	path := args[2]
	if isReadableFile(path) {
		fmt.Printf("Really delete file %s? (y/N): ", path)
		reader := bufio.NewReader(os.Stdin)
		confirmation, _ := reader.ReadString('\n')
		if strings.TrimSpace(confirmation) == "y" {
			err := os.Remove(path)
			if err != nil {
				fmt.Printf("Error deleting file: %s\n", err)
			} else {
				fmt.Printf("File %s deleted successfully\n", path)
			}
		} else {
			fmt.Println("File not deleted")
		}
	} else {
		fmt.Printf("Error: %s does not exist\n", path)
	}
}

func handleMove(args []string) {
	if len(args) < 4 {
		fmt.Println("Usage: ./td move <source-path> <destination-path>")
		return
	}

	source := args[2]
	destination := args[3]

	if !isReadableFile(source) {
		fmt.Printf("Error: %s does not exist\n", source)
		return
	}

	if isReadableFile(destination) {
		fmt.Printf("Error: %s already exists\n", destination)
		return
	}

	fmt.Printf("Really move file %s to %s? (y/N): ", source, destination)
	reader := bufio.NewReader(os.Stdin)
	confirmation, _ := reader.ReadString('\n')
	if strings.TrimSpace(confirmation) == "y" {
		err := os.Rename(source, destination)
		if err != nil {
			fmt.Printf("Error moving file: %s\n", err)
		} else {
			fmt.Printf("File moved successfully from %s to %s\n", source, destination)
		}
	} else {
		fmt.Println("File not moved")
	}
}

func isReadableFile(path string) bool {
	fileInfo, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	if fileInfo.IsDir() {
		return false
	}

	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	return true
}

func showHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  ./td show <path-to-file>  - Show the command and the file path")
	fmt.Println("  ./td new <path-to-file>   - Create a new file and open in Vim")
	fmt.Println("  ./td edit <path-to-file>  - Edit an existing file in Vim")
	fmt.Println("  ./td delete <path-to-file> - Delete a file after confirmation")
	fmt.Println("  ./td move <source> <destination> - Move a file after confirmation")
}
