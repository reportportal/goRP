package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/urfave/cli/v3"

	gorppkg "github.com/reportportal/goRP/v5/pkg/gorp"
)

const uuidParseRegex = `ReportPortal Launch UUID: ([a-fA-F\d]{8}-[a-fA-F\d]{4}-[a-fA-F\d]{4}-[a-fA-F\d]{4}-[a-fA-F\d]{12})`

// Arguments for quality gate check
var (
	argQualityGateTimeout = &cli.DurationFlag{
		Name:    "quality-gate-timeout",
		Aliases: []string{"qgt"},
		Usage:   "Timeout for quality gate check",
		Sources: cli.EnvVars("QUALITY_GATE_TIMEOUT"),
		Value:   1 * time.Minute,
	}
	argQualityGateCheckInterval = &cli.DurationFlag{
		Name:    "quality-gate-check-interval",
		Aliases: []string{"qgci"},
		Usage:   "Interval for quality gate check",
		Sources: cli.EnvVars("QUALITY_GATE_CHECK_INTERVAL"),
		Value:   3 * time.Second,
	}
)

// Quality gate command
var (
	qualityGateCommand = &cli.Command{
		Name:     "quality-gate",
		Aliases:  []string{"qg"},
		Usage:    "Quality gate commands",
		Commands: []*cli.Command{checkQualityGateCommand},
	}
	checkQualityGateCommand = &cli.Command{
		Name:    "check",
		Aliases: []string{"qgc"},
		Usage:   "Check the quality gate status of a launch",
		Flags: []cli.Flag{
			argQualityGateTimeout,
			argQualityGateCheckInterval,
		},
		MutuallyExclusiveFlags: []cli.MutuallyExclusiveFlags{
			// option launch-uuid cannot be set along with option stdin
			{
				Required: true,
				Category: "source",
				Flags: [][]cli.Flag{
					{
						&cli.StringFlag{
							Name:    "launch-uuid",
							Usage:   "Launch uuid to check the quality gate status for",
							Sources: cli.EnvVars("LAUNCH_UUID"),
						},
					},
					{
						&cli.BoolFlag{
							Name:  "stdin",
							Usage: "Parse stdin for launch uuid",
						},
					},
				},
			},
		},

		Action: checkQualityGate,
	}
)

// checkQualityGate is a command interface to check the quality gate status
func checkQualityGate(ctx context.Context, cmd *cli.Command) error {
	cfg, err := getConfig(cmd)
	if err != nil {
		return err
	}

	launchID := cmd.String("launch-uuid")
	if launchID == "" {
		if cmd.Bool("stdin") {
			if sErr := checkStdinEmpty(); sErr != nil {
				return err
			}

			uuidRegex := regexp.MustCompile(uuidParseRegex)
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				data := scanner.Text()
				found := uuidRegex.FindStringSubmatch(data)
				if len(found) == 2 {
					launchID = found[1]
					break
				}
			}
		}
	}
	fmt.Println(launchID)

	if launchID == "" {
		return cli.Exit("launch uuid not found", 1)
	}

	return checkQualityGateInternal(ctx, launchID, cfg, cmd)
}

// checkQualityGateInternal checks the quality gate status of a launch
func checkQualityGateInternal(ctx context.Context,
	launchID string,
	cfg *clientConfig,
	cmd *cli.Command,
) error {
	qgTimeout := cmd.Duration(argQualityGateTimeout.Name)
	qgCheckInterval := cmd.Duration(argQualityGateCheckInterval.Name)

	rpClient, _, err := buildClientFromConfig(ctx, cfg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, qgTimeout)
	defer cancel()

	checkF := func(ctx context.Context) (bool, error) {
		launchObject, _, err := rpClient.LaunchAPI.GetLaunch(ctx, launchID, cfg.Project).Execute()
		if err != nil {
			return true, err
		}

		qg, ok := gorppkg.ParseQualityGate(launchObject.GetMetadata())
		if !ok {
			return true, errors.New("quality gate metadata not found")
		}
		if qg.Status == "IN PROGRESS" {
			return false, nil
		}
		if qg.Status != "PASSED" {
			return true, fmt.Errorf("status: %s", qg.Status)
		}
		return true, nil
	}

	pollForStatusF := func(ctx context.Context) error {
		ticker := time.NewTicker(qgCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("timeout waiting for quality gate status")
			default:
				ok, cErr := checkF(context.Background())
				if cErr != nil {
					return cErr
				}
				if !ok {
					continue
				}
				return nil
			}
		}
	}
	if pErr := pollForStatusF(ctx); pErr != nil {
		return cli.Exit(fmt.Errorf("quality gate check error: %w", pErr), 10)
	}
	return nil
}
