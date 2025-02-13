package main

import (
	"client/commands"
	"log/slog"
	"os"

	"github.com/jessevdk/go-flags"
)

func main() {
	args := os.Args[1:]

	if err := run(args); err != nil {
		flagsErr, ok := err.(*flags.Error)
		if ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		}

		slog.Error("Main func exited", "err", err)
		os.Exit(1)
	}
}

type Flags struct {
}

func run(args []string) error {
	var parsedArgs Flags
	parser := flags.NewParser(&parsedArgs, flags.Default)

	_, err := parser.AddCommand("build",
		"Build expression",
		"Build nix expression",
		&commands.BuildCommand{})
	if err != nil {
		return err
	}

	_, err = parser.AddCommand("watch",
		"Watch store",
		"Watch store",
		&commands.WatchCommand{})
	if err != nil {
		return err
	}

	_, err = parser.ParseArgs(args)
	if err != nil {
		return err
	}

	return nil
}
