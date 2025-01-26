package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
)

func main() {
	ctx := context.Background()
	slog.SetLogLoggerLevel(slog.LevelDebug)
	if err := run(ctx, os.Args); err != nil {
		slog.ErrorContext(ctx, "Main func exited with error", "err", err)
	}
}

func run(ctx context.Context, args []string) error {
	slog.DebugContext(ctx, "Running agent", "args", args)
	if len(args) < 2 {
		return errors.New("Missing command")
	}
	cmd := args[1]
	if cmd == "build" {
		pkg := args[2]
		slog.InfoContext(ctx, "Building package", "pkg", pkg)
		cmd := exec.CommandContext(ctx, "nix", "build", ".#")
		slog.DebugContext(ctx, "Running command", "cmd", cmd.String())
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		slog.InfoContext(ctx, "Stdout", "msg", stdout.String())
		if err != nil {
			slog.ErrorContext(ctx, "Cmd failed", "err", stderr.String())
			return err
		}

		return nil
	}

	return nil
}
