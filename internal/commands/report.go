package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	gorppkg "github.com/reportportal/goRP/v5/pkg/gorp"
)

// agentName identifies gorp in the "agent" system attribute of the launches it reports.
const agentName = "gorp"

// scannerMaxLine bounds a single test2json line; output events can be larger than bufio's 64 KB default.
const scannerMaxLine = 10 * 1024 * 1024

var (
	reportCommand = &cli.Command{
		Name:  "report",
		Usage: "Reports input to report portal",
		Commands: []*cli.Command{
			reportTest2JsonCommand,
			startLaunchCommand,
			startTestCommand,
			reportLogCommand,
			finishTestCommand,
			finishLaunchCommand,
		},
	}

	reportTest2JsonCommand = &cli.Command{
		Name:  "test2json",
		Usage: "Input format: test2json",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "file",
				Aliases: []string{"f"},
				Usage:   "File Name",
				Sources: cli.EnvVars("FILE"),
			},
			&cli.StringFlag{
				Name:    "launchName",
				Aliases: []string{"ln"},
				Usage:   "Launch Name",
				Sources: cli.EnvVars("LAUNCH_NAME"),
				Value:   "gorp launch",
			},
			&cli.BoolFlag{
				Name:    "reportEmptyPkg",
				Aliases: []string{"ep"},
				Usage:   "Whether empty packages need to be reported. Default is false",
				Sources: cli.EnvVars("REPORT_EMPTY_PKG"),
				Value:   false,
			},
			attrFlag(
				"Launch attribute with format 'key:value'. Omitting a ':' separator will tag the launch with the value.",
			),
			&cli.StringFlag{
				Name:    "description",
				Usage:   "Launch description",
				Sources: cli.EnvVars("LAUNCH_DESCRIPTION"),
			},
			&cli.StringFlag{
				Name:    "mode",
				Usage:   "Launch mode: DEFAULT or DEBUG",
				Sources: cli.EnvVars("LAUNCH_MODE"),
				Value:   string(gorppkg.LaunchModes.Default),
			},
			&cli.BoolFlag{
				Name: "rerun",
				Usage: "Report into the latest launch with the same name instead of creating a new one " +
					"(the report fails if there is none)",
				Sources: cli.EnvVars("LAUNCH_RERUN"),
			},
			&cli.StringFlag{
				Name:    "rerun-of",
				Usage:   "UUID of the launch to rerun; implies --rerun",
				Sources: cli.EnvVars("LAUNCH_RERUN_OF"),
			},
			&cli.StringFlag{
				Name: "launch-uuid",
				Usage: "Report into this existing launch instead of starting a new one. " +
					"The launch is neither started nor finished: finish it with 'report finish-launch'",
			},
			&cli.BoolFlag{
				Name: "skipped-issue",
				Usage: "Whether skipped tests are 'To Investigate' (true, ReportPortal's default) " +
					"or 'No Defect' (false)",
				Sources: cli.EnvVars("SKIPPED_ISSUE"),
				Value:   true,
			},
			&cli.BoolFlag{
				Name: "retries",
				Usage: "Report a test that runs again in the same input (go test -count=N, gotestsum --rerun-fails) " +
					"as a retry of its previous run: only the last run is counted. " +
					"With false every run is a test item of its own",
				Sources: cli.EnvVars("REPORT_RETRIES"),
				Value:   true,
			},
			&cli.BoolFlag{
				Name:  "print-launch-uuid",
				Usage: "Print launch UUID and link to console",
				Value: false,
			},
			&cli.BoolFlag{
				Name:    "quality-gate-check",
				Aliases: []string{"qgc"},
				Usage:   "Check quality gate status. Exits with exit code 10 if quality gate check fails.",
				Sources: cli.EnvVars("QUALITY_GATE_CHECK"),
				Value:   false,
			},
			argQualityGateTimeout,
			argQualityGateCheckInterval,
		},
		Action: reportTest2Json,
	}
)

func reportTest2Json(ctx context.Context, cmd *cli.Command) error {
	cfg, err := getConfig(cmd)
	if err != nil {
		return err
	}
	if err = validateJoinFlags(cmd); err != nil {
		return err
	}

	// start reporting of the launch
	launchID, launchLink, err := reportLaunch(ctx, cfg, cmd)
	if err != nil {
		return err
	}
	// check if we need to print launch UUID (and the link, if the server returned one)
	if cmd.Bool("print-launch-uuid") && launchID != "" {
		_, _ = fmt.Fprintf(cmd.Writer, "ReportPortal Launch UUID: %s\n", launchID)
		if launchLink != "" {
			_, _ = fmt.Fprintf(cmd.Writer, "ReportPortal Launch: %s\n", launchLink)
		}
	}
	// check if we need to check quality gate
	if cmd.Bool("quality-gate-check") {
		if launchID == "" {
			return cli.Exit(
				"quality gate check skipped: no launch was started, the input has no test events",
				1,
			)
		}
		qgErr := checkQualityGateInternal(ctx, launchID, cfg, cmd)
		if qgErr != nil {
			return qgErr
		}
	}

	return nil
}

// reportLaunch reports the test2json input as a launch and returns its UUID and UI link.
func reportLaunch(
	ctx context.Context,
	cfg *clientConfig,
	cmd *cli.Command,
) (string, string, error) {
	// validate before anything is read or sent
	mode, err := test2jsonLaunchMode(cmd.String("mode"))
	if err != nil {
		return "", "", err
	}

	rpClient := buildReportingClient(ctx, cfg)

	// an existing launch is checked before the input is read: nothing can be reported into a launch
	// the server does not know
	joined, err := findLaunchToJoin(
		ctx, rpClient, cmd.String("launch-uuid"), joinLaunchWait, joinLaunchPollInterval,
	)
	if err != nil {
		return "", "", err
	}

	input := make(chan *testEvent)

	rep := newReporter(ctx, input, rpClient, cmd.String("launchName"), reporterOptions{
		reportEmpty:  cmd.Bool("reportEmptyPkg"),
		skippedIssue: cmd.Bool("skipped-issue"),
		agentVersion: agentVersion(cmd),
		attributes:   cmd.StringSlice("attr"),
		description:  cmd.String("description"),
		mode:         mode,
		rerun:        cmd.Bool("rerun"),
		rerunOf:      cmd.String("rerun-of"),
		joined:       joined,
		// a command without the flag (tests) reports every run on its own, as before the flag existed
		retries: cmd.Bool("retries"),
	})

	// start reporting in a separate goroutine.
	// recvErr is written before done is closed, so it is safe to read once done is closed.
	var recvErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		recvErr = rep.receive()
	}()

	readErr := feedEventsFrom(ctx, cmd, input, done)
	close(input)
	// wait for the reporter to flush logs and finish the launch
	<-done

	// reporter errors take precedence: they are the reason the input might have been abandoned
	if recvErr != nil {
		slog.Error("input processing interrupted", "error", recvErr)
		return "", "", recvErr
	}
	if readErr != nil {
		return "", "", readErr
	}
	return rep.launchUUID, rep.launchLink, nil
}

// launchStartFlags are the flags that describe a launch to start; they mean nothing for --launch-uuid.
var launchStartFlags = []string{"launchName", "attr", "description", "mode"}

// validateJoinFlags rejects the flag combinations that cannot work with --launch-uuid, before
// anything is read or sent.
func validateJoinFlags(cmd *cli.Command) error {
	if cmd.String("launch-uuid") == "" {
		return nil
	}
	if cmd.Bool("rerun") || cmd.String("rerun-of") != "" {
		return cli.Exit("--launch-uuid cannot be combined with --rerun or --rerun-of: "+
			"the launch exists already, there is nothing to start", 1)
	}
	if cmd.Bool("quality-gate-check") {
		return cli.Exit(
			"--launch-uuid cannot be combined with --quality-gate-check: the launch is not "+
				"finished by this command, and its quality gate is calculated only once it is finished. "+
				"Run 'gorp quality-gate check --launch-uuid <uuid>' after 'gorp report finish-launch'",
			1,
		)
	}
	for _, name := range launchStartFlags {
		if cmd.IsSet(name) {
			slog.Warn(
				"flag is ignored with --launch-uuid: the launch exists already",
				"flag",
				"--"+name,
			)
		}
	}
	return nil
}

const (
	// joinLaunchWait is how long a launch that was just started may stay unknown to the server:
	// launches are created asynchronously.
	joinLaunchWait = 10 * time.Second
	// joinLaunchPollInterval is the pause between the lookups of a launch that is not found yet.
	joinLaunchPollInterval = time.Second
)

// findLaunchToJoin looks up the launch of --launch-uuid; the result is nil when uuid is empty.
//
// The reporting API does not check the launch: it accepts items for an unknown launch UUID and drops
// them later, without telling the client. A launch that is not found is looked up again every
// interval for the wait time, since `report start-launch` returns before the server has created the
// launch. Any other error fails at once.
func findLaunchToJoin(
	ctx context.Context,
	client *gorppkg.ReportingClient,
	uuid string,
	wait, interval time.Duration,
) (*gorppkg.LaunchInfo, error) {
	if uuid == "" {
		return nil, nil //nolint:nilnil // no launch to join is not an error
	}
	// the wait limits the retries, not a lookup that is under way: a slow answer must not be
	// reported as a launch that does not exist
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		launch, err := client.GetLaunchByUUID(ctx, uuid)
		if err == nil {
			if launch.Status != launchStatusInProgress {
				slog.Warn("the launch is not in progress: ReportPortal accepts the results, "+
					"but does not update the status of a finished launch",
					"launch", uuid, "status", launch.Status)
			}
			return launch, nil
		}
		var httpErr *gorppkg.HTTPError
		if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusNotFound {
			return nil, fmt.Errorf("looking up launch %s: %w", uuid, err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, fmt.Errorf("launch %s not found: %w", uuid, err)
		case <-ticker.C:
		}
	}
}

// test2jsonLaunchMode validates the --mode value of `report test2json` (case-insensitive). ReportPortal
// answers an unknown mode with a 400; failing here names the flag and the accepted values instead.
func test2jsonLaunchMode(value string) (gorppkg.LaunchMode, error) {
	switch mode := gorppkg.LaunchMode(strings.ToUpper(strings.TrimSpace(value))); mode {
	case "":
		return gorppkg.LaunchModes.Default, nil
	case gorppkg.LaunchModes.Default, gorppkg.LaunchModes.Debug:
		return mode, nil
	default:
		return "", cli.Exit(fmt.Sprintf("invalid --mode %q: expected %s or %s",
			value, gorppkg.LaunchModes.Default, gorppkg.LaunchModes.Debug), 1)
	}
}

// agentVersion is gorp's version as set on the root command ("1.2.3 (date)"), without the date.
func agentVersion(cmd *cli.Command) string {
	if fields := strings.Fields(cmd.Root().Version); len(fields) > 0 {
		return fields[0]
	}
	return "dev"
}

// feedEventsFrom feeds the reporter from the --file argument, or from stdin if it is not set.
func feedEventsFrom(
	ctx context.Context,
	cmd *cli.Command,
	input chan<- *testEvent,
	done <-chan struct{},
) error {
	if fileName := cmd.String("file"); fileName != "" {
		f, err := os.Open(filepath.Clean(fileName))
		if err != nil {
			return err
		}
		defer func() {
			if cErr := f.Close(); cErr != nil {
				slog.Error(cErr.Error())
			}
		}()
		return feedEvents(ctx, f, input, done)
	}

	return feedEvents(ctx, os.Stdin, input, done)
}

// feedEvents reads test2json events from r and sends them to the reporter.
// It stops early when the reporter has exited (done is closed) or ctx is canceled.
func feedEvents(
	ctx context.Context,
	r io.Reader,
	input chan<- *testEvent,
	done <-chan struct{},
) error {
	events, scanErr := scanEvents(ctx, r)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			return nil
		case ev, ok := <-events:
			if !ok {
				return <-scanErr
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-done:
				return nil
			case input <- ev: // send event to reporter
			}
		}
	}
}

// scanEvents decodes test2json lines from r in a separate goroutine, so that a blocked read
// (stdin without data) doesn't prevent the caller from reacting to cancellation.
// The events channel is closed when r is exhausted or ctx is canceled; the outcome is then
// available from the error channel. A goroutine blocked in Read is abandoned until r is closed
// or the process exits.
func scanEvents(ctx context.Context, r io.Reader) (<-chan *testEvent, <-chan error) {
	events := make(chan *testEvent)
	result := make(chan error, 1)

	go func() {
		var err error
		defer func() {
			result <- err
			close(events)
		}()

		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), scannerMaxLine)
		for scanner.Scan() {
			var ev testEvent
			if jsonErr := json.Unmarshal(scanner.Bytes(), &ev); jsonErr != nil {
				// build tools may interleave plain text with the JSON stream
				slog.Warn("skipping non-JSON input line", "error", jsonErr)
				continue
			}
			select {
			case events <- &ev:
			case <-ctx.Done():
				return
			}
		}
		if scanErr := scanner.Err(); scanErr != nil {
			err = fmt.Errorf("reading input: %w", scanErr)
		}
	}()

	return events, result
}
