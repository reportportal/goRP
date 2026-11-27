package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
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
			&cli.BoolFlag{
				Name: "skipped-issue",
				Usage: "Whether skipped tests are 'To Investigate' (true, ReportPortal's default) " +
					"or 'No Defect' (false)",
				Sources: cli.EnvVars("SKIPPED_ISSUE"),
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
	rpClient := buildReportingClient(ctx, cfg)

	input := make(chan *testEvent)

	rep := newReporter(ctx, input, rpClient, cmd.String("launchName"), reporterOptions{
		reportEmpty:  cmd.Bool("reportEmptyPkg"),
		skippedIssue: cmd.Bool("skipped-issue"),
		agentVersion: agentVersion(cmd),
		attributes:   cmd.StringSlice("attr"),
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
