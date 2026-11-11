package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/urfave/cli/v3"

	gorppkg "github.com/reportportal/goRP/v5/pkg/gorp"
)

const uuidParseRegex = `ReportPortal Launch UUID: ([a-fA-F\d]{8}-[a-fA-F\d]{4}-[a-fA-F\d]{4}-[a-fA-F\d]{4}-[a-fA-F\d]{12})`

var uuidRegex = regexp.MustCompile(uuidParseRegex)

// ReportPortal quality gate status values.
const (
	qgStatusInProgress = "IN PROGRESS"
	qgStatusPassed     = "PASSED"
)

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
				return sErr
			}

			var fErr error
			if launchID, fErr = findLaunchUUID(ctx, os.Stdin); fErr != nil {
				return fErr
			}
		}
	}
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

	// checkF returns done=true once polling must stop: the quality gate is resolved
	// (err is nil if it passed) or the request can never succeed.
	// Otherwise (not done) a non-nil err is transient, e.g. the metadata is not yet
	// available right after the launch has finished, and is retried until timeout.
	checkF := func(ctx context.Context) (bool, error) {
		launchObject, resp, err := rpClient.LaunchAPI.GetLaunch(ctx, launchID, cfg.Project).
			Execute()
		if err != nil {
			return isPermanentHTTPStatus(resp), err
		}

		qg, ok := gorppkg.ParseQualityGate(launchObject.GetMetadata())
		if !ok {
			return false, errors.New("quality gate metadata not found")
		}
		if qg.Status == qgStatusInProgress {
			return false, nil
		}
		if qg.Status != qgStatusPassed {
			return true, fmt.Errorf("status: %s", qg.Status)
		}
		return true, nil
	}

	pollForStatusF := func(ctx context.Context) error {
		ticker := time.NewTicker(qgCheckInterval)
		defer ticker.Stop()

		var lastErr error
		for {
			done, cErr := checkF(ctx)
			if done {
				return cErr
			}
			// an error caused by our own deadline would hide the real reason
			if cErr != nil && ctx.Err() == nil {
				slog.Warn("quality gate check failed, will retry", "error", cErr)
				lastErr = cErr
			}

			select {
			case <-ctx.Done():
				if lastErr != nil {
					return fmt.Errorf(
						"timeout waiting for quality gate status: %w (last error: %w)",
						ctx.Err(),
						lastErr,
					)
				}
				return fmt.Errorf("timeout waiting for quality gate status: %w", ctx.Err())
			case <-ticker.C:
			}
		}
	}
	if pErr := pollForStatusF(ctx); pErr != nil {
		return cli.Exit(fmt.Errorf("quality gate check error: %w", pErr), 10)
	}
	return nil
}

// isPermanentHTTPStatus reports whether retrying a request that got this response is pointless:
// a client error (wrong API key, project or launch) other than the retryable 408 and 429.
func isPermanentHTTPStatus(resp *http.Response) bool {
	if resp == nil {
		return false
	}
	code := resp.StatusCode

	return code >= http.StatusBadRequest && code < http.StatusInternalServerError &&
		code != http.StatusRequestTimeout && code != http.StatusTooManyRequests
}

// findLaunchUUID scans r for the line printed by `--print-launch-uuid` and returns the UUID, or an
// empty string if there is none. A blocked read is abandoned (until r is closed or the process
// exits) when ctx is cancelled.
func findLaunchUUID(ctx context.Context, r io.Reader) (string, error) {
	type result struct {
		id  string
		err error
	}
	resCh := make(chan result, 1)

	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			if found := uuidRegex.FindStringSubmatch(scanner.Text()); len(found) == 2 {
				resCh <- result{id: found[1]}
				return
			}
		}
		if err := scanner.Err(); err != nil {
			resCh <- result{err: fmt.Errorf("reading stdin: %w", err)}
			return
		}
		resCh <- result{}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-resCh:
		return res.id, res.err
	}
}
