package main

import (
	"os"

	"github.com/nsxbet/sql-reviewer/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
