package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"

	rp "github.com/reportportal/goRP/v5/internal/commands"
)

var (
	version = "dev"
	date    = "unknown"
)

func main() {
	root := cli.Command{
		Name:    "goRP",
		Usage:   "ReportPortal CLI Client",
		Version: fmt.Sprintf("%s (%s)", version, date),
		Authors: []any{"Andrei Varabyeu <andrei.varabyeu@gmail.com>"},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			// configure logging
			slog.SetDefault(slog.New(
				slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
					Level:     slog.LevelError,
					AddSource: true,
				}),
			))
			return ctx, nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "uuid",
				Aliases: []string{"u"},
				Usage:   "Access Token",
				Sources: cli.EnvVars("GORP_UUID"),
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
