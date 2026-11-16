package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

var errFilterNotProvided = errors.New("either IDs or filter must be provided")

// newLaunchCommand builds the `launch` command tree. A constructor (unlike the package-level
// commands) gives callers fresh flag instances, which keeps tests independent of each other.
func newLaunchCommand() *cli.Command {
	listLaunchesCommand := &cli.Command{
		Name:  "list",
		Usage: "List launches",
		MutuallyExclusiveFlags: []cli.MutuallyExclusiveFlags{
			{
				Flags: [][]cli.Flag{
					{filterNameFlag()},
					{
						&cli.StringSliceFlag{
							Name:    "filter",
							Aliases: []string{"f"},
							Usage:   "Filter",
							Sources: cli.EnvVars("FILTER"),
						},
					},
				},
			},
		},
		Action: listLaunches,
	}

	mergeCommand := &cli.Command{
		Name:   "merge",
		Usage:  "Merge Launches",
		Action: mergeLaunches,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "name",
				Aliases:  []string{"n"},
				Usage:    "New Launch Name",
				Sources:  cli.EnvVars("MERGE_LAUNCH_NAME"),
				Required: true,
			},
			&cli.StringFlag{
				Name:    "type",
				Aliases: []string{"t"},
				Usage:   "Merge Type: BASIC or DEEP",
				Sources: cli.EnvVars("MERGE_TYPE"),
				Value:   string(gorp.MergeTypes.Deep),
				Validator: func(v string) error {
					if v != string(gorp.MergeTypes.Deep) && v != string(gorp.MergeTypes.Basic) {
						return fmt.Errorf("unknown merge type %q, expected BASIC or DEEP", v)
					}

					return nil
				},
			},
		},
		// exactly one way of selecting the launches to merge
		MutuallyExclusiveFlags: []cli.MutuallyExclusiveFlags{
			{
				Required: true,
				Category: "launches",
				Flags: [][]cli.Flag{
					{
						&cli.Int64SliceFlag{
							Name:    "ids",
							Usage:   "Launch IDS to Merge",
							Sources: cli.EnvVars("MERGE_LAUNCH_IDS"),
						},
					},
					{
						&cli.StringFlag{
							Name:    "filter",
							Aliases: []string{"f"},
							Usage:   "Launches Filter",
							Sources: cli.EnvVars("MERGE_LAUNCH_FILTER"),
						},
					},
					{filterNameFlag()},
				},
			},
		},
	}

	return &cli.Command{
		Name:     "launch",
		Usage:    "Operations over launches",
		Commands: []*cli.Command{listLaunchesCommand, mergeCommand},
	}
}

func mergeLaunches(ctx context.Context, cmd *cli.Command) error {
	rpClient, cfg, err := buildClient(ctx, cmd)
	if err != nil {
		return err
	}

	ids, err := getMergeIDs(ctx, cmd, rpClient, cfg.Project)
	if err != nil {
		return err
	}
	rq := openapi.MergeLaunchesRQ{
		Name:      cmd.String("name"),
		MergeType: cmd.String("type"),
		Launches:  ids,
	}
	launchResource, _, err := rpClient.LaunchAsyncAPI.MergeLaunchesOldUuid(ctx, cfg.Project).
		MergeLaunchesRQ(rq).
		Execute()
	if err != nil {
		return fmt.Errorf("unable to merge launches: %w", err)
	}

	_, _ = fmt.Fprintln(cmd.Writer, launchResource.Id)

	return nil
}

func listLaunches(ctx context.Context, cmd *cli.Command) error {
	rpClient, cfg, err := buildClient(ctx, cmd)
	if err != nil {
		return err
	}

	var launches *openapi.PageLaunchViewModel

	if filters := cmd.StringSlice("filter"); len(filters) > 0 {
		filter := strings.Join(filters, "&")
		launches, err = rpClient.GetLaunchesByFilterString(ctx, cfg.Project, filter)
	} else if filterName := cmd.String("filter-name"); filterName != "" {
		launches, err = rpClient.GetLaunchesByFilterName(ctx, cfg.Project, filterName)
	} else {
		launches, _, err = rpClient.LaunchAPI.GetProjectLaunches(ctx, cfg.Project).Execute()
	}
	if err != nil {
		return err
	}

	for _, launch := range launches.Content {
		_, _ = fmt.Fprintf(cmd.Writer, "%d #%d %q\n", launch.Id, launch.Number, launch.Name)
	}

	return nil
}

func getMergeIDs(
	ctx context.Context,
	cmd *cli.Command,
	rpClient *gorp.Client,
	project string,
) ([]int64, error) {
	if ids := cmd.Int64Slice("ids"); len(ids) > 0 {
		return ids, nil
	}

	var launches []openapi.LaunchViewModel

	switch filter, filterName := cmd.String("filter"), cmd.String("filter-name"); {
	case filter != "":
		// all pages: merging only the first page would silently skip launches
		var err error
		if launches, err = rpClient.GetAllLaunchesByFilterString(ctx, project, filter); err != nil {
			return nil, fmt.Errorf("unable to find launches by filter: %w", err)
		}
	case filterName != "":
		page, err := rpClient.GetLaunchesByFilterName(ctx, project, filterName)
		if err != nil {
			return nil, fmt.Errorf("unable to find launches by filter: %w", err)
		}
		launches = page.Content
	default:
		return nil, errFilterNotProvided
	}

	ids := make([]int64, len(launches))
	for i, l := range launches {
		ids[i] = l.Id
	}

	return ids, nil
}
