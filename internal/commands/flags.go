package commands

import (
	"context"

	"github.com/urfave/cli/v3"
)

// Flags shared by several commands. They are constructors, so every command owns its flag instances.

func launchUUIDFlag() cli.Flag {
	return &cli.StringFlag{
		Name:     "launch-uuid",
		Usage:    "Launch UUID",
		Sources:  cli.EnvVars("LAUNCH_UUID"),
		Required: true,
	}
}

func itemUUIDFlag(required bool) cli.Flag {
	return &cli.StringFlag{
		Name:     "item-uuid",
		Usage:    "Test item UUID",
		Sources:  cli.EnvVars("ITEM_UUID"),
		Required: required,
	}
}

func statusFlag(usage string) cli.Flag {
	return &cli.StringFlag{Name: "status", Usage: usage}
}

func attrFlag(usage string) cli.Flag {
	return &cli.StringSliceFlag{Name: "attr", Aliases: []string{"a"}, Usage: usage}
}

func filterNameFlag() cli.Flag {
	return &cli.StringFlag{
		Name:    "filter-name",
		Aliases: []string{"fn"},
		Usage:   "Name of a saved ReportPortal filter",
		Sources: cli.EnvVars("FILTER_NAME"),
	}
}

// stepFunc is the testable core of a command: it gets the resolved configuration.
type stepFunc[T any] func(ctx context.Context, cfg *clientConfig, cmd *cli.Command) (T, error)

// withConfig turns a step into a CLI action that loads the configuration first and discards the
// step's result (steps print what the user needs to cmd.Writer themselves).
func withConfig[T any](step stepFunc[T]) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		cfg, err := getConfig(cmd)
		if err != nil {
			return err
		}
		_, err = step(ctx, cfg, cmd)

		return err
	}
}
