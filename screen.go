package main

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
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
