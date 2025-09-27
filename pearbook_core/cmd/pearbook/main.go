package main

import (
	"os"

	"github.com/khelechy/pearbook/pearbook_core/cmd/pearbook/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
