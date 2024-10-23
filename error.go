package main

import (
	"errors"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"os"
)

func exitWithError(err error) {
	fmt.Println(err.Error())
	os.Exit(1)
}

type userErr struct {
	msg string
}

func (e userErr) Error() string {
	return e.msg
}

func handleError(err error, screen tcell.Screen) {
	var userErr *userErr
	if errors.As(err, &userErr) {
		renderError(err.Error(), screen)
	} else {
		exitWithError(err)
	}
}
