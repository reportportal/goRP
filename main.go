package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

	rp "github.com/reportportal/goRP/v5/internal/commands"
)

var (
	version = "dev"
	date    = "unknown"
)

func main() {
	root := cli.Command{
		Name:                  "goRP",
		Usage:                 "ReportPortal CLI Client",
		EnableShellCompletion: true,
		Version:               fmt.Sprintf("%s (%s)", version, date),
		Authors:               []any{"Andrei Varabyeu <andrei.varabyeu@gmail.com>"},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			// configure logging
			var level slog.Level
			if err := level.UnmarshalText([]byte(cmd.String("log-level"))); err != nil {
				return ctx, err
			}
			slog.SetDefault(slog.New(
				slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
					Level:     level,
					AddSource: true,
				}),
			))

			// Deprecation warnings for the old --uuid flag and GORP_UUID env var.
			if os.Getenv("GORP_UUID") != "" {
				slog.Warn("GORP_UUID is deprecated and will be removed in a future release; use GORP_API_KEY instead")
			}
			for _, arg := range os.Args[1:] {
				if arg == "--uuid" || strings.HasPrefix(arg, "--uuid=") {
					slog.Warn("--uuid is deprecated and will be removed in a future release; use --api-key instead")
					break
				}
			}

			return ctx, nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "api-key",
				Aliases: []string{"u", "uuid"},
				Usage:   "API Key (user token)",
				Sources: cli.EnvVars("GORP_API_KEY", "GORP_UUID"),
			},
			&cli.StringFlag{
				Name:    "project",
				Aliases: []string{"p"},
				Usage:   "ReportPortal Project Name",
				Sources: cli.EnvVars("GORP_PROJECT"),
			},

			&cli.StringFlag{
				Name:  "host",
				Usage: "ReportPortal Server Name",
			},
			&cli.StringFlag{
				Name:  "log-level",
				Usage: "Logging level" + slog.LevelKey,
				Value: "debug",
			},
		},
		Commands: rp.RootCommand,
	}

	defer func() {
		if r := recover(); r != nil {
			log.Fatalf("error: %v", r)
		}
	}()
	if err := root.Run(context.Background(), os.Args); err != nil {
		//nolint:gocritic
		log.Fatal(err.Error())
	}
}
