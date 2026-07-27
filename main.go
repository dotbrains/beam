package main

import (
	"os"

	"github.com/dotbrains/beam/cmd"
)

var version = "dev"

func main() {
	if err := cmd.Execute(version); err != nil {
		os.Exit(cmd.ExitCode(err))
	}
}
