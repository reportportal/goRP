package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/reportportal/goRP/pkg/ginkgo/rpagent"

	"github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

// Version information — set at build time with -ldflags.
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func boolPtr(b bool) *bool { return &b }

func ptr[T any](v T) *T { return &v }

// run is the real entry point. It returns an exit code.
// Using a separate function (rather than inlining into main) ensures that all
// deferred calls (cleanup, CloseRPLogger) run before os.Exit is invoked.
func run() int {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "help", "-h", "--help", "-help":
			if len(os.Args) > 2 {
				showTopicHelp(os.Args[2])
			} else {
				showMainHelp()
			}
			return 0
		case "version", "--version", "-version":
			showVersion()
			return 0
		case "bootstrap", "generate", "outline", "unfocus", "build", "watch", "labels":
			fmt.Fprintf(os.Stderr, "ginkgo-rp does not support the %q subcommand.\n", os.Args[1])
			fmt.Fprintf(
				os.Stderr,
				"ginkgo-rp only wraps test execution (run/parallel/focus/etc).\n",
			)
			fmt.Fprintf(os.Stderr, "For dev-time tooling, use plain ginkgo:\n")
			fmt.Fprintf(os.Stderr, "  ginkgo %s\n", strings.Join(os.Args[1:], " "))
			return 1
		default:
			if typo, suggestion := detectTypo(os.Args[1]); typo {
				fmt.Fprintf(os.Stderr, "Unknown command: %s\n%s\n", os.Args[1], suggestion)
				return 1
			}
		}
		for _, arg := range os.Args[1:] {
			if arg == "-h" || arg == "--help" || arg == "-help" {
				showMainHelp()
				return 0
			}
		}
	}

	sessionID := fmt.Sprintf("%d-%s", time.Now().Unix(), uuid.New().String())

	if err := rpagent.InitializeRPLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: could not initialize rp-agent.log: %v\n", err)
	}
	defer rpagent.CloseRPLogger()
	defer cleanup(sessionID)

	// Listen from the start, so a signal during the launch setup (or while ginkgo compiles)
	// still lets the launch be closed instead of killing the wrapper and leaving it open.
	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigs)

	wantReport := shouldFinalizeLaunch(os.Args[1:])
	strict := strictMode()

	if wantReport {
		fmt.Printf(
			"======== Starting ReportPortal-enabled Ginkgo run | Session: %s ========\n",
			sessionID,
		)
	}

	// reporting is true only when the launch and its suites really exist. If
	// ReportPortal cannot be reached, the tests still run and the exit code stays
	// Ginkgo's (unless RP_STRICT=true asks for the run to fail instead).
	var (
		launchID  string
		reporting bool
	)
	if wantReport {
		fmt.Printf("\n")
		fmt.Printf(
			"================================================================================\n",
		)
		fmt.Printf(
			"                    REPORTPORTAL LAUNCH INITIALIZATION                         \n",
		)
		fmt.Printf(
			"================================================================================\n",
		)
		fmt.Printf("\n")
		ctx, cancel := context.WithTimeout(context.Background(), launchStartTimeout)
		id, _, err := createRPLaunchAndSaveState(ctx, sessionID, os.Args[1:])
		cancel()
		if err != nil {
			if strict {
				fmt.Fprintf(os.Stderr, "\nERROR: Failed to create ReportPortal launch: %v\n", err)
				return 1
			}
			fmt.Fprintf(
				os.Stderr,
				"\nWARNING: ReportPortal reporting is OFF for this run: %v\n",
				err,
			)
			fmt.Fprintf(os.Stderr, "         The tests will still run, but nothing is reported. "+
				"Set RP_STRICT=true to fail instead.\n\n")
		} else {
			reporting = true
			launchID = id

			// Only set RP_SESSION_ID after the launch (and state file) exist.
			// This prevents workers from trying to load a missing state file when
			// --dry-run or other non-test commands are used.
			_ = os.Setenv("RP_SESSION_ID", sessionID)
			// Workers run in their own package directory, which may be outside this
			// tree, so tell them which properties file the wrapper used.
			if path, _, ok := rpagent.ConfigFilePath(); ok {
				if abs, aerr := filepath.Abs(path); aerr == nil {
					_ = os.Setenv(rpagent.ConfigFileEnv, abs)
				}
			}

			fmt.Printf("\n  Launch initialization complete.\n")
			fmt.Printf("  All test processes will use this launch.\n")
			fmt.Printf("\n")
			fmt.Printf(
				"================================================================================\n",
			)
		}
	}

	ginkgoPath, err := exec.LookPath("ginkgo")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: ginkgo not found in PATH. Install with:\n")
		fmt.Fprintf(os.Stderr, "  go install github.com/onsi/ginkgo/v2/ginkgo@latest\n")
		if reporting && launchID != "" {
			abortLaunch(context.Background(), sessionID, launchID)
		}
		return 1
	}

	cwd, _ := os.Getwd()
	args := buildGinkgoArgs(os.Args[1:], agentSkipPackages(cwd))
	passthrough := len(os.Args) > 1 && passthroughSubcommands[os.Args[1]]

	if reporting {
		fmt.Printf("\n")
		fmt.Printf(
			"================================================================================\n",
		)
		fmt.Printf(
			"                        EXECUTING GINKGO TESTS                                  \n",
		)
		fmt.Printf(
			"================================================================================\n",
		)
		fmt.Printf("\n  Command: %s %v\n\n", ginkgoPath, args)
		fmt.Printf(
			"================================================================================\n\n",
		)
	} else if wantReport {
		fmt.Printf("\n  Command (reporting off): %s %v\n\n", ginkgoPath, args)
	} else if !passthrough {
		fmt.Printf("\n  [dry-run] Command: %s %v\n\n", ginkgoPath, args)
	}

	select {
	case sig := <-sigs: // interrupted before ginkgo started
		if reporting && launchID != "" {
			abortLaunch(context.Background(), sessionID, launchID)
		}
		fmt.Fprintf(os.Stderr, "\n  Interrupted (%v) before the tests started.\n", sig)
		return signalCode(sig)
	default:
	}

	cmd := exec.Command(ginkgoPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	isolate(cmd)
	var interrupted os.Signal
	exitCode := 0
	if err = cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: failed to execute ginkgo: %v\n", err)
		exitCode = 1
	} else {
		// Ctrl-Z stays handled until the run ends, finalization included.
		defer watchJobControl(cmd)()
		interrupted, err = waitForwardingSignals(cmd, sigs)
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			} else {
				fmt.Fprintf(os.Stderr, "ERROR: failed to execute ginkgo: %v\n", err)
				exitCode = 1
			}
		}
		if interrupted != nil {
			exitCode = signalCode(interrupted)
			fmt.Fprintf(
				os.Stderr,
				"\n  Interrupted (%v): finalizing ReportPortal launch...\n",
				interrupted,
			)
		}
	}

	if reporting {
		fmt.Printf("\n")
		fmt.Printf(
			"================================================================================\n",
		)
		fmt.Printf(
			"                    FINALIZING REPORTPORTAL LAUNCH                             \n",
		)
		fmt.Printf(
			"================================================================================\n",
		)
		exitCode = finalizeRun(sigs, sessionID, launchID, exitCode, strict)
	}

	if reporting {
		fmt.Printf(
			"\n================================================================================\n",
		)
		if exitCode == 0 {
			fmt.Printf(
				"                    RUN COMPLETED SUCCESSFULLY                                  \n",
			)
		} else {
			fmt.Printf(
				"                    RUN COMPLETED WITH FAILURES                                 \n",
			)
		}
		fmt.Printf(
			"================================================================================\n\n",
		)
	}

	return exitCode
}

// finalizeRun finishes the launch once ginkgo has exited, and returns the exit code to use:
// ginkgo's own, unless the run was interrupted while finalizing, or RP_STRICT asks for a
// reporting failure to fail the run.
//
// Finalization is bounded (RP_FINALIZE_TIMEOUT) and can be interrupted. If it cannot finish,
// because ReportPortal hangs, keeps failing, or the user pressed Ctrl-C, the launch is
// stopped instead, so it does not stay In Progress.
func finalizeRun(
	sigs <-chan os.Signal,
	sessionID, launchID string,
	exitCode int,
	strict bool,
) int {
	timeout := finalizeTimeout()
	finalizeCtx, cancelFinalize := context.WithTimeout(context.Background(), timeout)
	defer cancelFinalize()
	// A second signal cancels this one too, which cuts the fallback short.
	hardCtx, cancelHard := context.WithCancel(context.Background())
	defer cancelHard()
	stopWatching := watchSignals(sigs, cancelFinalize, cancelHard)

	fmt.Printf("\n  Step 1: Waiting for all test processes to complete...\n")

	// Workers have already exited (cmd.Wait returned). The wait is only
	// needed to let the OS finish flushing NDJSON writes; cap it short.
	if err := waitForWorkersToFinish(finalizeCtx, sessionID); err != nil {
		fmt.Printf("\n  WARNING: %v\n  Continuing with available results...\n", err)
	}

	fmt.Printf("\n  Step 2: Collecting and analyzing test results...\n\n")

	err := aggregateAndFinalizeLaunch(finalizeCtx, exitCode != 0, sessionID)
	if err != nil {
		// Whatever went wrong, the launch must not be left In Progress: stop it, as the last
		// resort. (If the server is gone, this fails too; it is bounded and best-effort.)
		switch {
		case errors.Is(finalizeCtx.Err(), context.DeadlineExceeded):
			fmt.Fprintf(os.Stderr, "WARNING: finalizing took longer than %s (%s). "+
				"Stopping the launch instead.\n", timeout, finalizeTimeoutEnv)
		case finalizeCtx.Err() == nil:
			fmt.Fprintf(os.Stderr, "WARNING: %v. Stopping the launch instead.\n", err)
		}
		abortLaunch(hardCtx, sessionID, launchID)
	}
	if sig := stopWatching(); sig != nil {
		return signalCode(sig)
	}

	if err != nil {
		if strict {
			fmt.Fprintf(os.Stderr, "ERROR: failed to aggregate and finalize launch: %v\n", err)
			return 2
		}
		// The tests ran; their result must not be hidden by a reporting problem.
		fmt.Fprintf(os.Stderr, "WARNING: could not finalize the ReportPortal launch: %v\n", err)
		fmt.Fprintf(
			os.Stderr,
			"         The exit code is Ginkgo's. Set RP_STRICT=true to fail instead.\n",
		)
	}
	return exitCode
}

// waitForwardingSignals waits for the started command, forwarding SIGINT/SIGTERM (and SIGHUP,
// sent when a terminal closes) to it
// so Ginkgo can stop gracefully. A second signal kills it. It returns the first signal
// received, if any, so the caller can still finalize the launch before exiting.
func waitForwardingSignals(cmd *exec.Cmd, sigs <-chan os.Signal) (os.Signal, error) {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var first os.Signal
	for {
		select {
		case err := <-done:
			return first, err
		case sig := <-sigs:
			if first == nil {
				first = sig
				forwardSignal(cmd, sig)
			} else {
				killGroup(cmd)
			}
		}
	}
}

// launchStartTimeout bounds the whole launch-and-suites setup, so an unreachable
// ReportPortal costs seconds, not minutes, before the tests start.
const launchStartTimeout = 30 * time.Second

// finalizeTimeoutEnv names the variable that bounds how long finishing the launch may take.
const finalizeTimeoutEnv = "RP_FINALIZE_TIMEOUT"

const (
	// defaultFinalizeTimeout bounds finalizing the launch after the tests ran. Each request
	// already has its own timeout, but a ReportPortal that hangs mid-run would otherwise
	// keep a run with hundreds of containers waiting for hours.
	defaultFinalizeTimeout = 3 * time.Minute

	// maxConsecutiveFinishFailures is how many finish requests in a row may fail before the
	// wrapper stops finishing items one by one and closes the launch as STOPPED instead.
	maxConsecutiveFinishFailures = 3
)

// stopLaunchTimeout bounds the last-resort stopLaunch. A variable so tests can shorten it.
var stopLaunchTimeout = 30 * time.Second

// errReportPortalNotResponding is returned once finish requests keep failing.
var errReportPortalNotResponding = errors.New("ReportPortal is not responding")

// finalizeTimeout returns the finalization deadline: RP_FINALIZE_TIMEOUT as a Go duration
// ("90s", "5m"), or the default when it is unset or not a positive duration.
func finalizeTimeout() time.Duration {
	v := os.Getenv(finalizeTimeoutEnv)
	if v == "" {
		return defaultFinalizeTimeout
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		fmt.Fprintf(
			os.Stderr,
			"WARNING: ignoring %s=%q (want a duration such as 90s or 5m); using %s\n",
			finalizeTimeoutEnv,
			v,
			defaultFinalizeTimeout,
		)
		return defaultFinalizeTimeout
	}
	return d
}

// failureStreak counts finish requests that failed one after another because the server
// seems to be gone.
type failureStreak struct{ n int }

// note records the outcome of one request. It returns errReportPortalNotResponding once
// maxConsecutiveFinishFailures requests in a row have failed.
//
// Only a connection error, a timeout or an HTTP 5xx counts as a failure. An HTTP 4xx answer
// (an item ReportPortal rejects, a missing item, rate limiting) shows the server is up, so
// like a success it ends the streak instead of adding to it: three rejected items must not
// be reported as "ReportPortal is not responding".
func (f *failureStreak) note(err error) error {
	var httpErr *gorp.HTTPError
	if err == nil ||
		(errors.As(err, &httpErr) && httpErr.StatusCode < http.StatusInternalServerError) {
		f.n = 0
		return nil
	}
	f.n++
	if f.n >= maxConsecutiveFinishFailures {
		return errReportPortalNotResponding
	}
	return nil
}

// watchSignals reacts to signals while the launch is being finalized. The first one cancels
// finalization (the wrapper then stops the launch); a second one cancels that too, so Ctrl-C
// always gets the wrapper out. It returns a function that stops watching and reports the
// first signal received, if any.
func watchSignals(
	sigs <-chan os.Signal,
	cancelFinalize, cancelAll context.CancelFunc,
) (stop func() os.Signal) {
	var (
		mu    sync.Mutex
		first os.Signal
	)
	quit := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-quit:
				return
			case sig := <-sigs:
				mu.Lock()
				isFirst := first == nil
				if isFirst {
					first = sig
				}
				mu.Unlock()
				if isFirst {
					fmt.Fprintf(os.Stderr, "\n  Interrupted (%v): stopping the launch. "+
						"Interrupt again to give up at once.\n", sig)
					cancelFinalize()
				} else {
					cancelAll()
				}
			}
		}
	}()
	return func() os.Signal {
		close(quit)
		<-done
		mu.Lock()
		defer mu.Unlock()
		return first
	}
}

// strictMode reports whether RP_STRICT asks reporting failures to fail the run.
func strictMode() bool {
	v, err := strconv.ParseBool(os.Getenv("RP_STRICT"))
	return err == nil && v
}

func main() {
	os.Exit(run())
}

// abortLaunch finishes a launch as STOPPED (its suites first, see stopLaunch) when the
// wrapper cannot finalize it normally after StartLaunch has been called. This prevents the
// launch from staying IN_PROGRESS indefinitely in ReportPortal.
func abortLaunch(ctx context.Context, sessionID, launchID string) {
	coordinator, err := rpagent.NewLaunchCoordinator(sessionID)
	if err != nil {
		return
	}
	state, err := coordinator.LoadAgentState()
	if err != nil {
		return
	}
	config, err := rpagent.LoadRPConfig()
	if err != nil {
		return
	}
	client := rpagent.NewReportingClient(state.Endpoint, state.Project, config.APIKey, config.Debug)
	var suiteIDs []string
	if sc, err := rpagent.LoadSuiteCache(sessionID); err == nil {
		for _, id := range sc.Suites {
			suiteIDs = append(suiteIDs, id)
		}
	}
	stopLaunch(ctx, client, launchID, suiteIDs)
}

// stopLaunch marks a launch STOPPED through FinishLaunch. goRP's StopLaunch uses an
// endpoint that does not exist on current ReportPortal servers. The suites are finished
// first: ReportPortal turns a STOPPED launch into FAILED while any suite is still open.
//
// It is the last resort, so it is bounded (stopLaunchTimeout) and ctx can cut it short.
// The suites get two thirds of the time, so a hanging server cannot also eat the time
// the launch itself needs.
func stopLaunch(
	ctx context.Context,
	client *gorp.ReportingClient,
	launchID string,
	suiteIDs []string,
) {
	status := string(gorp.Statuses.Stopped)
	suitesCtx, cancelSuites := context.WithTimeout(ctx, stopLaunchTimeout*2/3)
	defer cancelSuites()
	for _, id := range suiteIDs {
		if suitesCtx.Err() != nil {
			break
		}
		_, _ = client.FinishTest(suitesCtx, id, &openapi.FinishTestItemRQ{
			EndTime:    time.Now(),
			Status:     &status,
			LaunchUuid: launchID,
		})
	}
	launchCtx, cancelLaunch := context.WithTimeout(ctx, stopLaunchTimeout/3)
	defer cancelLaunch()
	_, _ = client.FinishLaunch(launchCtx, launchID, &openapi.FinishExecutionRQ{
		EndTime: time.Now(),
		Status:  &status,
	})
}

func cleanup(sessionID string) {
	if len(os.Args) > 1 && passthroughSubcommands[os.Args[1]] {
		return
	}
	fmt.Printf("\n  Cleaning up coordination files for session: %s\n", sessionID)
	tempDir := os.TempDir()

	// Clean only this session's files — never glob for all rp_cache_* or
	// rp-agent-attachments-* because concurrent runs on the same host would
	// delete each other's files.
	for _, pattern := range []string{
		fmt.Sprintf("rp_agent_state_%s.json", sessionID),
		fmt.Sprintf("rp_report_index_%s.json", sessionID),
		fmt.Sprintf("rp_status_%s.json", sessionID),
		fmt.Sprintf("rp_status_%s_*.json", sessionID),
		fmt.Sprintf("rp_done_%s_*.flag", sessionID),
		fmt.Sprintf("rp_suites_%s.json", sessionID),
	} {
		matches, _ := filepath.Glob(filepath.Join(tempDir, pattern))
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}
	// The agent puts the session ID into these names (InitializeAttachments and
	// ContainerCacheManager.GetOrCreate), so the patterns are exact: they match this run's
	// attachment directories and container cache files (and their .lock/.tmp leftovers).
	for _, pattern := range []string{
		fmt.Sprintf("rp-agent-attachments-%s-*", sessionID),
		fmt.Sprintf("rp_cache_%s_*", sessionID),
	} {
		matches, _ := filepath.Glob(filepath.Join(tempDir, pattern))
		for _, m := range matches {
			_ = os.RemoveAll(m)
		}
	}
	fmt.Printf("  Cleanup completed.\n")
}

// waitForWorkersToFinish polls for done-flags up to a short timeout.
// Workers have already exited when this is called (cmd.Run returned); this
// gives the OS a moment to flush pending file writes.
func waitForWorkersToFinish(ctx context.Context, sessionID string) error {
	flagPattern := filepath.Join(os.TempDir(), fmt.Sprintf("rp_done_%s_*.flag", sessionID))
	maxWaitTime := 5 * time.Second
	pollInterval := 200 * time.Millisecond
	startTime := time.Now()

	for {
		flags, _ := filepath.Glob(flagPattern)
		if len(flags) > 0 {
			fmt.Printf("\r  Found %d worker report(s)... All workers completed.   \n", len(flags))
			return nil
		}
		if time.Since(startTime) >= maxWaitTime {
			return fmt.Errorf(
				"no worker done-flags found after %v (compile error or crash?)",
				maxWaitTime,
			)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func shouldFinalizeLaunch(args []string) bool {
	if len(args) == 0 {
		return true
	}
	nonTestCommands := []string{
		"version",
		"help",
		"bootstrap",
		"generate",
		"unfocus",
		"outline",
		"labels",
		"build",
		"watch",
	}
	firstArg := args[0]
	if firstArg == "help" {
		return false
	}
	for _, cmd := range nonTestCommands {
		if firstArg == cmd {
			return false
		}
	}
	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "-help" ||
			arg == "--labels" || arg == "--version" || arg == "--build-only" {
			return false
		}
		// --dry-run and --dry-run=true skip launch creation; --dry-run=false does not.
		if arg == "--dry-run" || arg == "--dry-run=true" {
			return false
		}
	}
	return true
}

// pass-throughSubcommands are Ginkgo subcommands that don't run tests and
// must not have --skip-package injected (they either don't accept it or
// position it wrongly relative to their own arguments).
var passthroughSubcommands = map[string]bool{
	"outline": true, "bootstrap": true, "generate": true,
	"unfocus": true, "watch": true, "build": true, "labels": true,
}

// buildGinkgoArgs rewrites os.Args[1:] for the ginkgo child process:
//   - For test-running subcommands: merges rpagent into --skip-package
//   - For utility subcommands (outline, bootstrap, etc.): passes args unchanged
func buildGinkgoArgs(args, skip []string) []string {
	// Utility subcommands: pass through unchanged — they don't accept --skip-package.
	if len(args) > 0 && passthroughSubcommands[args[0]] {
		return args
	}

	pa := splitArgs(args)

	// Only add what the user has not already excluded.
	var missing []string
	have := strings.Split(pa.SkipPackage, ",")
	for _, tok := range skip {
		if !slices.Contains(have, tok) {
			missing = append(missing, tok)
		}
	}
	flags := pa.Flags
	if len(missing) > 0 {
		flags = mergeSkipPackage(flags, strings.Join(missing, ","))
	}

	// Ginkgo requires all flags before the package paths, so flags typed after a path
	// are moved up front.
	var out []string
	if pa.Subcommand != "" {
		out = append(out, pa.Subcommand)
	}
	out = append(out, flags...)
	out = append(out, pa.Positional...)
	return append(out, pa.Tail...)
}

// mergeSkipPackage appends extra to the user's --skip-package value, or adds the flag.
func mergeSkipPackage(flags []string, extra string) []string {
	out := make([]string, 0, len(flags)+1)
	merged := false
	for i := 0; i < len(flags); i++ {
		name, val, hasEq := strings.Cut(strings.TrimLeft(flags[i], "-"), "=")
		isSkip := strings.HasPrefix(flags[i], "-") &&
			(name == "skip-package" || name == "skipPackage")
		switch {
		case isSkip && hasEq:
			out = append(out, "--skip-package="+val+","+extra)
			merged = true
		case isSkip && i+1 < len(flags):
			out = append(out, "--skip-package="+flags[i+1]+","+extra)
			merged = true
			i++
		default:
			out = append(out, flags[i])
		}
	}
	if !merged {
		out = append(out, "--skip-package="+extra)
	}
	return out
}

// agentSkipPackages returns the --skip-package filters needed to keep Ginkgo from
// running this agent's own rpagent package as a test suite. That only matters when
// the agent's source is inside the tree being scanned (developing the agent itself);
// for every other project it returns nothing, so no user package is ever skipped.
//
// Ginkgo matches a filter with strings.Contains against the suite path relative to the
// working directory ("./rpagent"), so the filter is the full relative path of our
// package, never the bare word "rpagent".
func agentSkipPackages(cwd string) []string {
	if mod, ok := modulePath(filepath.Join(cwd, "go.mod")); ok &&
		mod == "github.com/reportportal/goRP/pkg/ginkgo" {
		return []string{"./rpagent"} // running from the agent's own module root
	}
	if fi, err := os.Stat(
		filepath.Join(cwd, "pkg", "ginkgo", "rpagent"),
	); err == nil &&
		fi.IsDir() {
		return []string{"./pkg/ginkgo/rpagent"} // running from the goRP repo root
	}
	return nil
}

// modulePath returns the module path declared in a go.mod file.
func modulePath(goMod string) (string, bool) {
	data, err := os.ReadFile(goMod)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`), true
		}
	}
	return "", false
}

// isPackagePath reports whether arg looks like a package path rather than a flag.
// Package paths start with ./ or ../, end with /, or equal ./... or ...
// Anything starting with - is a flag, not a path.
func createRPLaunchAndSaveState(
	ctx context.Context,
	sessionID string,
	args []string,
) (launchID string, suiteCache *rpagent.SuiteCache, retErr error) {
	config, err := rpagent.LoadRPConfig()
	if err != nil {
		return "", nil, fmt.Errorf("could not load config (reportportal.properties): %w", err)
	}

	client := rpagent.NewReportingClient(
		config.Endpoint,
		config.Project,
		config.APIKey,
		config.Debug,
	)
	if config.Debug {
		fmt.Printf("  HTTP Logging: rp-agent.log (enabled)\n")
	}
	fmt.Printf("\n")

	// Discover suites BEFORE creating the launch so a discovery error does not
	// leave an orphan IN_PROGRESS launch.
	pa := splitArgs(args)
	targets := pa.targets()
	cwd, _ := os.Getwd()
	opts := discoverOpts{
		SkipPackages: append(strings.Split(pa.SkipPackage, ","), agentSkipPackages(cwd)...),
	}
	if pa.Tags != "" {
		opts.Tags = strings.Split(pa.Tags, ",")
	}

	var dirs []string
	for _, t := range targets {
		dirs = append(dirs, t.Dir)
	}
	fmt.Printf("  Suite Discovery:\n")
	fmt.Printf("    - Targets:   %s\n", strings.Join(dirs, ", "))
	fmt.Printf("    - Recursive: %v\n", pa.Recursive)
	if pa.Tags != "" {
		fmt.Printf("    - Tags:      %s\n", pa.Tags)
	}
	fmt.Printf("\n  Scanning for test suites...\n")

	var suites []DiscoveredSuite
	seen := map[string]bool{}
	for _, t := range targets {
		found, err := discoverSuites(t, opts)
		if err != nil {
			// A listed folder with no suite is not an error while others may have one.
			fmt.Fprintf(os.Stderr, "  WARNING: %v\n", err)
			continue
		}
		for _, f := range found {
			if !seen[f.Directory] { // overlapping targets (e.g. "." and "./a")
				seen[f.Directory] = true
				suites = append(suites, f)
			}
		}
	}
	if len(suites) == 0 {
		return "", nil, fmt.Errorf("no Ginkgo suites found in %s", strings.Join(dirs, ", "))
	}
	fmt.Printf("  Found %d suite(s)\n\n", len(suites))

	launchRQ := &openapi.StartLaunchRQ{
		Uuid:      uuid.NewString(),
		Name:      config.LaunchName,
		StartTime: time.Now(),
		Mode:      ptr(config.Mode),
	}
	if config.Description != "" {
		launchRQ.Description = ptr(config.Description)
	}
	for _, a := range config.Attributes {
		attr := openapi.ItemAttributesRQ{Value: a.Value}
		if a.Key != "" {
			attr.Key = ptr(a.Key)
		}
		launchRQ.Attributes = append(launchRQ.Attributes, attr)
	}
	// A system attribute (hidden in the UI) that records how this launch treated skipped
	// specs, as the Java agents do, so ReportPortal can tell the launches apart.
	launchRQ.Attributes = append(launchRQ.Attributes, openapi.ItemAttributesRQ{
		Key:    ptr("skippedIssue"),
		Value:  strconv.FormatBool(config.SkippedIssue),
		System: ptr(true),
	})

	fmt.Printf("  Launch Configuration:\n")
	fmt.Printf("    - Name:    %s\n", config.LaunchName)
	fmt.Printf("    - Project: %s\n", config.Project)
	fmt.Printf("    - Mode:    %s\n", config.Mode)
	fmt.Printf("\n  Creating launch...\n")

	launchRS, err := client.StartLaunch(ctx, launchRQ)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create launch: %w", err)
	}
	if launchRS.Id == nil || *launchRS.Id == "" {
		return "", nil, fmt.Errorf(
			"launch created but returned empty ID — check endpoint/project/api key",
		)
	}
	launchID = *launchRS.Id
	fmt.Printf("  Launch ID: %s\n\n", launchID)

	sc := &rpagent.SuiteCache{Suites: make(map[string]string)}

	// From this point on, any error must finish/stop the launch to prevent orphans.
	defer func() {
		if retErr != nil && launchID != "" {
			var ids []string
			for _, id := range sc.Suites {
				ids = append(ids, id)
			}
			stopLaunch(context.Background(), client, launchID, ids)
		}
	}()

	fmt.Printf("  Creating suites in ReportPortal:\n\n")
	for i, suite := range suites {
		suiteRS, err := client.StartTest(ctx, &openapi.StartTestItemRQ{
			Uuid:       uuid.NewString(),
			Name:       suite.Name,
			StartTime:  time.Now(),
			LaunchUuid: launchID,
			HasStats:   boolPtr(true),
			Type:       string(gorp.TestItemTypes.Suite),
		})
		if err != nil {
			return launchID, nil, fmt.Errorf("failed to create suite %q: %w", suite.Name, err)
		}
		if suiteRS.Id == nil || *suiteRS.Id == "" {
			return launchID, nil, fmt.Errorf("suite %q created but returned empty ID", suite.Name)
		}
		// Store the symlink-resolved path so workers (which also resolve symlinks) match.
		resolvedDir := suite.Directory
		if r, rerr := filepath.EvalSymlinks(suite.Directory); rerr == nil {
			resolvedDir = r
		}
		sc.Suites[resolvedDir] = *suiteRS.Id
		fmt.Printf("    [%d/%d] Suite: %s\n", i+1, len(suites), suite.Name)
		fmt.Printf("            Path:  %s\n", suite.RelativePath)
		fmt.Printf("            ID:    %s\n\n", *suiteRS.Id)
	}

	if err := rpagent.SaveSuiteCache(sessionID, sc); err != nil {
		return launchID, nil, fmt.Errorf("failed to save suite cache: %w", err)
	}
	fmt.Printf("  Suite cache saved: %d suite(s)\n", len(sc.Suites))

	// Write state file. APIKey is intentionally excluded — workers re-read it
	// from reportportal.properties to avoid storing a plaintext credential.
	statePath := filepath.Join(os.TempDir(), fmt.Sprintf(rpagent.StateFilePattern, sessionID))
	state := rpagent.AgentState{
		Endpoint:   config.Endpoint,
		Project:    config.Project,
		LaunchName: config.LaunchName,
		LaunchID:   launchID,
		SessionID:  sessionID,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return launchID, nil, fmt.Errorf("failed to marshal state: %w", err)
	}
	if err := os.WriteFile(statePath, data, 0o600); err != nil {
		return launchID, nil, fmt.Errorf("failed to write state file: %w", err)
	}
	fmt.Printf("  Agent state saved: %s\n", filepath.Base(statePath))
	return launchID, sc, nil
}

func aggregateAndFinalizeLaunch(
	ctx context.Context,
	hasSuiteFailures bool,
	sessionID string,
) error {
	if sessionID == "" {
		sessionID = os.Getenv("RP_SESSION_ID")
	}
	if sessionID == "" {
		return fmt.Errorf("RP_SESSION_ID not set")
	}

	coordinator, err := rpagent.NewLaunchCoordinator(sessionID)
	if err != nil {
		return err
	}

	resultPattern := filepath.Join(os.TempDir(), fmt.Sprintf("rp_status_%s_*.json", sessionID))
	reportFilePaths, err := filepath.Glob(resultPattern)
	if err != nil {
		return fmt.Errorf("failed to glob for report files: %w", err)
	}
	if len(reportFilePaths) == 0 {
		fmt.Printf("WARNING: no report files found (pattern: %s).\n", resultPattern)
	}

	var allStatuses []rpagent.SharedSpecStatus
	for _, reportPath := range reportFilePaths {
		file, err := os.Open(reportPath)
		if err != nil {
			fmt.Printf("WARNING: could not read %s: %v\n", reportPath, err)
			continue
		}

		// Use json.Decoder instead of bufio.Scanner to avoid the 64 KB line limit.
		dec := json.NewDecoder(file)
		linesRead := 0
		for {
			var status rpagent.SharedSpecStatus
			if decErr := dec.Decode(&status); decErr != nil {
				if decErr != io.EOF {
					fmt.Printf("WARNING: JSON decode error in %s after %d records: %v\n",
						filepath.Base(reportPath), linesRead, decErr)
				}
				break
			}
			allStatuses = append(allStatuses, status)
			linesRead++
		}
		_ = file.Close()
		rpagent.LogVerboseOperation(
			fmt.Sprintf("loaded %d specs from %s", linesRead, filepath.Base(reportPath)),
		)
		_ = os.Remove(reportPath)
	}

	fmt.Printf(
		"\n================================================================================\n",
	)
	fmt.Printf("                        TEST EXECUTION SUMMARY                                  \n")
	fmt.Printf(
		"================================================================================\n\n",
	)
	// Failed BeforeSuite/AfterSuite nodes fail the suite and launch but are not tests: keep
	// them out of the counts and list them separately.
	specs, fixtures := splitFixtureResults(allStatuses)
	fmt.Printf("  Total Tests Executed: %d\n\n", len(specs))

	passed, failed, interrupted, skipped := 0, 0, 0, 0
	priorityStats := make(map[int]map[string]int)

	for _, status := range specs {
		norm := strings.ToLower(status.Status)
		var cat string
		switch norm {
		case "passed":
			passed++
			cat = "passed"
		case "failed", "panicked":
			failed++
			cat = "failed"
		case "interrupted", "aborted", "timedout":
			interrupted++
			cat = "failed" // counts toward launch failure
		case "skipped", "pending":
			skipped++
			cat = "skipped"
		default:
			fmt.Fprintf(
				os.Stderr,
				"WARNING: unrecognized status %q for %s\n",
				status.Status,
				status.SpecName,
			)
			cat = "skipped"
		}
		if priorityStats[status.Priority] == nil {
			priorityStats[status.Priority] = make(map[string]int)
		}
		priorityStats[status.Priority][cat]++
		priorityStats[status.Priority]["total"]++
	}

	fmt.Printf("  Test Results:\n")
	fmt.Printf("    - Passed:      %d\n", passed)
	fmt.Printf("    - Failed:      %d\n", failed)
	fmt.Printf("    - Interrupted: %d\n", interrupted)
	fmt.Printf("    - Skipped:     %d\n\n", skipped)

	totalFailed := failed + interrupted

	if totalFailed > 0 {
		fmt.Printf(
			"--------------------------------------------------------------------------------\n",
		)
		fmt.Printf(
			"                            FAILED TESTS                                        \n",
		)
		fmt.Printf(
			"--------------------------------------------------------------------------------\n\n",
		)
		for _, status := range specs {
			norm := strings.ToLower(status.Status)
			if norm == "failed" || norm == "panicked" || norm == "interrupted" ||
				norm == "aborted" ||
				norm == "timedout" {
				fmt.Printf("  Test: %s\n", status.SpecName)
				fmt.Printf("    Status:   %s\n", strings.ToUpper(status.Status))
				if status.FailureLocation != "" {
					fmt.Printf("    Location: %s\n", status.FailureLocation)
				}
				if status.FailureMessage != "" {
					fmt.Printf("    Message:  %s\n", status.FailureMessage)
				}
				fmt.Printf("\n")
			}
		}
	}

	if len(fixtures) > 0 {
		fmt.Printf(
			"--------------------------------------------------------------------------------\n",
		)
		fmt.Printf(
			"                    SUITE SETUP / TEARDOWN FAILURES                             \n",
		)
		fmt.Printf(
			"--------------------------------------------------------------------------------\n\n",
		)
		for _, f := range fixtures {
			fmt.Printf("  %s\n", f.SpecName)
			fmt.Printf("    Suite:    %s\n", f.SuiteName)
			if f.FailureLocation != "" {
				fmt.Printf("    Location: %s\n", f.FailureLocation)
			}
			if f.FailureMessage != "" {
				fmt.Printf("    Message:  %s\n", f.FailureMessage)
			}
			fmt.Printf("\n")
		}
	}

	fmt.Printf("  Overall Status:\n")
	resultLines, forceFailed := overallResult(
		hasSuiteFailures,
		totalFailed,
		passed,
		skipped,
		len(fixtures),
	)
	for _, line := range resultLines {
		fmt.Println(line)
	}
	fmt.Println()
	if forceFailed {
		totalFailed = 1 // force launch to be marked failed
	}

	if len(priorityStats) > 1 {
		fmt.Printf(
			"--------------------------------------------------------------------------------\n",
		)
		fmt.Printf(
			"                        PRIORITY BREAKDOWN                                      \n",
		)
		fmt.Printf(
			"--------------------------------------------------------------------------------\n\n",
		)
		for p := 0; p <= 4; p++ {
			if stats, ok := priorityStats[p]; ok && stats["total"] > 0 {
				fmt.Printf("  P%d: total=%d passed=%d failed=%d skipped=%d\n",
					p, stats["total"], stats["passed"], stats["failed"], stats["skipped"])
			}
		}
		fmt.Printf("\n")
	}

	fmt.Printf(
		"================================================================================\n\n",
	)

	if err := coordinator.SetAggregatedResults(allStatuses); err != nil {
		return fmt.Errorf("failed to set aggregated results: %w", err)
	}

	if err := finishSuitesFromResults(ctx, sessionID, allStatuses, hasSuiteFailures); err != nil {
		return fmt.Errorf("failed to finish suites: %w", err)
	}

	if err := rpagent.FinalizeLaunch(ctx, hasSuiteFailures || totalFailed > 0); err != nil {
		return fmt.Errorf("failed to finalize launch: %w", err)
	}

	return nil
}

func finishSuitesFromResults(
	ctx context.Context,
	sessionID string,
	allStatuses []rpagent.SharedSpecStatus,
	hasSuiteFailures bool,
) error {
	suiteCache, err := rpagent.LoadSuiteCache(sessionID)
	if err != nil {
		return fmt.Errorf("failed to load suite cache: %w", err)
	}

	coordinator, err := rpagent.NewLaunchCoordinator(sessionID)
	if err != nil {
		return err
	}
	agentState, err := coordinator.LoadAgentState()
	if err != nil {
		return fmt.Errorf("failed to load agent state: %w", err)
	}
	config, err := rpagent.LoadRPConfig()
	if err != nil {
		return fmt.Errorf("failed to load config for API key: %w", err)
	}

	client := rpagent.NewReportingClient(
		agentState.Endpoint,
		agentState.Project,
		config.APIKey,
		config.Debug,
	)

	suiteResults := make(map[string][]rpagent.SharedSpecStatus)
	for _, status := range allStatuses {
		dir := status.SuiteDirectory
		// Resolve symlinks to match the keys stored by the wrapper.
		if r, rerr := filepath.EvalSymlinks(dir); rerr == nil {
			dir = r
		}
		suiteResults[dir] = append(suiteResults[dir], status)
	}

	fmt.Printf("--------------------------------------------------------------------------------\n")
	fmt.Printf("                        SUITE FINALIZATION                                      \n")
	fmt.Printf(
		"--------------------------------------------------------------------------------\n\n",
	)

	// Containers (Describe/Context) are shared by all specs and workers beneath them, so
	// they can only be finished here. Left alone, ReportPortal closes them when the launch
	// finishes: every container then ends at the launch's finish time with a status it
	// guesses (it marks a container whose only spec was skipped as FAILED).
	suiteIDs := make(map[string]bool, len(suiteCache.Suites))
	for _, id := range suiteCache.Suites {
		suiteIDs[id] = true
	}
	// Finish requests that keep failing mean ReportPortal is gone: stop instead of waiting
	// out every remaining item (the caller then stops the launch).
	streak := &failureStreak{}
	if err := finishContainers(
		ctx,
		client,
		agentState.LaunchID,
		rpagent.AggregateContainers(allStatuses, suiteIDs),
		streak,
	); err != nil {
		return err
	}

	fmt.Printf("  Finishing %d suite(s)...\n\n", len(suiteCache.Suites))

	for suiteDir, suiteID := range suiteCache.Suites {
		if err := ctx.Err(); err != nil {
			return err
		}
		results := suiteResults[suiteDir]
		suiteStatus := gorp.Statuses.Passed

		// Test gap: needs a mock RP server; covered by the real-RP sanity run.
		notRunNote := ""
		if len(results) == 0 {
			// Nothing was reported: the package never ran (earlier suite failed without
			// --keep-going, compile error, crash) or every spec was filtered out.
			// Never report that as FAILED or PASSED; it is simply not run.
			suiteStatus = gorp.Statuses.Skipped
			notRunNote = "Not run: no specs were reported for this suite " +
				"(an earlier suite failed, the package did not compile, or all specs were filtered out)."
			if hasSuiteFailures {
				notRunNote += " Use --keep-going to run remaining suites after a failure."
			}
			fmt.Printf("  Suite: %s — %s (not run)\n", filepath.Base(suiteDir), suiteStatus)
		} else {
			for _, r := range results {
				norm := strings.ToLower(r.Status)
				if norm == "failed" || norm == "panicked" || norm == "interrupted" ||
					norm == "aborted" ||
					norm == "timedout" {
					suiteStatus = gorp.Statuses.Failed
					break
				}
			}
			specs, _ := splitFixtureResults(results)
			fmt.Printf(
				"  Suite: %s — %s (%d tests)\n",
				filepath.Base(suiteDir),
				suiteStatus,
				len(specs),
			)
		}

		// A suite ends when its last spec did, not when this wrapper got around to it.
		suiteEnd := rpagent.LatestEnd(results)
		if suiteEnd.IsZero() {
			suiteEnd = time.Now()
		}
		if notRunNote != "" {
			if _, lerr := client.SaveLog(ctx, &openapi.SaveLogRQ{
				ItemUuid:   ptr(suiteID),
				LaunchUuid: agentState.LaunchID,
				Level:      ptr("INFO"),
				Time:       suiteEnd,
				Message:    ptr(notRunNote),
			}); lerr != nil {
				fmt.Printf("  WARNING: failed to log not-run note for %s: %v\n", suiteID, lerr)
			}
		}
		_, err := client.FinishTest(ctx, suiteID, &openapi.FinishTestItemRQ{
			EndTime:    suiteEnd,
			Status:     ptr(string(suiteStatus)),
			LaunchUuid: agentState.LaunchID,
		})
		if err != nil {
			fmt.Printf("  WARNING: failed to finish suite %s: %v\n", suiteID, err)
		}
		if gerr := streak.note(err); gerr != nil {
			return gerr
		}
	}
	fmt.Printf("\n")
	return nil
}

func showVersion() {
	modulePath := "github.com/reportportal/goRP/pkg/ginkgo"
	moduleVersion := Version

	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Path == modulePath && info.Main.Version != "(devel)" &&
			info.Main.Version != "" {
			moduleVersion = info.Main.Version
		}
		if moduleVersion == Version {
			for _, dep := range info.Deps {
				if dep.Path == modulePath {
					moduleVersion = dep.Version
					break
				}
			}
		}
	}
	if moduleVersion == Version {
		if v := getGitVersion(); v != "" {
			moduleVersion = v
		}
	}

	fmt.Printf("ginkgo-rp version %s\n", moduleVersion)
	fmt.Printf("ReportPortal Agent for Ginkgo\n")
	fmt.Printf("Module: %s\n", modulePath)
	if BuildTime != "unknown" {
		fmt.Printf("Built: %s\n", BuildTime)
	}
	if GitCommit != "unknown" {
		fmt.Printf("Commit: %s\n", GitCommit)
	}
	fmt.Println()
	fmt.Println("Features:")
	fmt.Println("  - Real-time test reporting")
	fmt.Println("  - Priority levels (P0-P4)")
	fmt.Println("  - Test reference IDs")
	fmt.Println("  - File attachments (streaming, 10 MB limit)")
	fmt.Println("  - Parameterized / table-driven tests (key='value' entry format)")
	fmt.Println("  - Parallel execution (-p / --procs N)")
	fmt.Println("  - By() step hierarchy with log routing")
	fmt.Println()
}

// detectTypo returns true and a suggestion when the first arg looks like a
// mistyped built-in command rather than a ginkgo flag or package path.
// Only intercepts args that start with "-" or match known command names closely,
// so legitimate ginkgo flags like "-r" still pass through.
func detectTypo(arg string) (bool, string) {
	typos := map[string]string{
		// help variants
		"--h":   "Did you mean: ginkgo-rp --help",
		"-he":   "Did you mean: ginkgo-rp --help",
		"--hel": "Did you mean: ginkgo-rp --help",
		"hlep":  "Did you mean: ginkgo-rp help",
		"halp":  "Did you mean: ginkgo-rp help", //nolint:misspell // deliberate typo that users make
		// version variants
		"--v":       "Did you mean: ginkgo-rp --version  (or -v for verbose ginkgo output)",
		"--ver":     "Did you mean: ginkgo-rp --version",
		"--versoin": "Did you mean: ginkgo-rp --version", //nolint:misspell // deliberate typo that users make
		"verison":   "Did you mean: ginkgo-rp version",   //nolint:misspell // deliberate typo that users make
		"vesrion":   "Did you mean: ginkgo-rp version",
	}
	if suggestion, ok := typos[arg]; ok {
		return true, suggestion
	}
	return false, ""
}

func getGitVersion() string {
	cmd := exec.Command("git", "describe", "--tags", "--always")
	if output, err := cmd.Output(); err == nil {
		return strings.TrimSpace(string(output))
	}
	return ""
}

func showMainHelp() {
	fmt.Println()
	fmt.Println("==================================================")
	fmt.Println("ginkgo-rp — ReportPortal Agent for Ginkgo")
	fmt.Println("==================================================")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  ginkgo-rp [FLAGS] [PACKAGES]")
	fmt.Println("  ginkgo-rp [COMMAND]")
	fmt.Println()
	fmt.Println("DESCRIPTION:")
	fmt.Println("  ginkgo-rp wraps test execution with integrated ReportPortal reporting.")
	fmt.Println("  Creates a launch, reports each test item in real time, and finalizes")
	fmt.Println("  the launch when all suites complete.")
	fmt.Println()
	fmt.Println("NOTE:")
	fmt.Println("  ginkgo-rp only covers test execution. Dev-time ginkgo subcommands")
	fmt.Println("  (bootstrap, generate, outline, build, watch, unfocus, labels) are")
	fmt.Println("  not supported — run those with plain ginkgo instead.")
	fmt.Println()
	fmt.Println("COMMANDS:")
	fmt.Println("  version   Show ginkgo-rp version")
	fmt.Println("  help      Show this help message")
	fmt.Println("  help [topic]  Detailed help for a specific topic")
	fmt.Println()
	fmt.Println("HELP TOPICS:")
	fmt.Println("  priority, reference, attachment, debug, parallel, parameterized, config")
	fmt.Println()
	fmt.Println("ESSENTIAL FLAGS (passed to ginkgo):")
	fmt.Println("  -r, --recursive       Run all suites under current directory")
	fmt.Println("  --keep-going          Continue after suite failures (recommended for CI)")
	fmt.Println("  -p, --parallel        Run in parallel (auto-detect CPU count)")
	fmt.Println("  --procs N             Run with N parallel processes")
	fmt.Println("  -v, --verbose         Enable Ginkgo verbose output")
	fmt.Println("  --focus REGEXP        Run only matching tests")
	fmt.Println("  --skip REGEXP         Skip matching tests")
	fmt.Println()
	fmt.Println("CONFIGURATION:")
	fmt.Println("  Create reportportal.properties in your project root.")
	fmt.Println("  Run 'ginkgo-rp help config' for the full list of options.")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  ginkgo-rp -r --keep-going")
	fmt.Println("  ginkgo-rp -r -p --keep-going -v")
	fmt.Println("  ginkgo-rp --focus \"smoke\"")
	fmt.Println()
}

func showTopicHelp(topic string) {
	switch topic {
	case "priority":
		showPriorityHelp()
	case "reference":
		showReferenceHelp()
	case "attachment":
		showAttachmentHelp()
	case "debug":
		showDebugHelp()
	case "parallel":
		showParallelHelp()
	case "parameterized":
		showParameterizedHelp()
	case "config":
		showConfigHelp()
	default:
		fmt.Printf("Unknown help topic: %s\n\n", topic)
		fmt.Println(
			"Available topics: priority, reference, attachment, debug, parallel, parameterized, config",
		)
	}
}

func showPriorityHelp() {
	fmt.Println()
	fmt.Println("Test Priorities — P0 to P4")
	fmt.Println()
	fmt.Println("LEVELS:")
	fmt.Println("  P0 - Critical  : blocking, core functionality")
	fmt.Println("  P1 - High      : important features")
	fmt.Println("  P2 - Medium    : standard features")
	fmt.Println("  P3 - Low       : minor features")
	fmt.Println("  P4 - Lowest    : cosmetic / nice-to-have")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println(`  import rpagent "github.com/reportportal/goRP/pkg/ginkgo/rpagent"`)
	fmt.Println()
	fmt.Println(`  It("should login", Label(rpagent.Priority(0)), func() { ... })`)
	fmt.Println()
	fmt.Println("NOTES:")
	fmt.Println(
		"  - A spec without a priority label has no priority in ReportPortal; the summary counts it as P1.",
	)
	fmt.Println("  - If multiple priority labels are set, leaf label takes precedence.")
	fmt.Println()
}

func showReferenceHelp() {
	fmt.Println()
	fmt.Println("Test Reference IDs — Linking to External Test Management")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println(`  import rpagent "github.com/reportportal/goRP/pkg/ginkgo/rpagent"`)
	fmt.Println()
	fmt.Println(`  It("should process payment",`)
	fmt.Println(`      Label(rpagent.TestReference("PAY-TC-001")), func() { ... })`)
	fmt.Println()
	fmt.Println("NOTES:")
	fmt.Println(
		"  - Combine with Priority: Label(rpagent.Priority(0), rpagent.TestReference(\"ID\"))",
	)
	fmt.Println("  - If multiple reference labels are set, the first is used.")
	fmt.Println()
}

func showAttachmentHelp() {
	fmt.Println()
	fmt.Println("File Attachments")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println(`  import rpagent "github.com/reportportal/goRP/pkg/ginkgo/rpagent"`)
	fmt.Println()
	fmt.Println(`  It("should return valid response", func() {`)
	fmt.Println(`      rpagent.AddAttachment("screenshot.png", "Login page")`)
	fmt.Println(`      rpagent.AddAttachment("response.json", "API response")`)
	fmt.Println(`  })`)
	fmt.Println()
	fmt.Println("NOTES:")
	fmt.Println("  - Must be called inside an It block.")
	fmt.Println("  - Maximum file size: 10 MB.")
	fmt.Println("  - Files are streamed (not fully loaded into memory).")
	fmt.Println()
}

func showDebugHelp() {
	fmt.Println()
	fmt.Println("Debug Logging")
	fmt.Println()
	fmt.Println("  Ginkgo -v flag       → Ginkgo verbose output only")
	fmt.Println("  rp.debug = true      → ReportPortal HTTP debug logging (in rp-agent.log)")
	fmt.Println()
	fmt.Println("  These are independent; you can enable either or both.")
	fmt.Println()
	fmt.Println("CONFIGURATION:")
	fmt.Println("  reportportal.properties:")
	fmt.Println("    rp.debug = true")
	fmt.Println()
}

func showParallelHelp() {
	fmt.Println()
	fmt.Println("Parallel Execution")
	fmt.Println()
	fmt.Println("  ginkgo-rp -p -r --keep-going")
	fmt.Println("  ginkgo-rp --procs 4 -r --keep-going")
	fmt.Println()
	fmt.Println("  --keep-going is strongly recommended in CI so that all suites")
	fmt.Println("  run and are reported even when some fail.")
	fmt.Println()
	fmt.Println("HOW PARALLEL DEDUPLICATION WORKS:")
	fmt.Println("  When N workers all hit the same Describe block, only one POSTs it to RP.")
	fmt.Println("  The winner claims an O_EXCL lock file, creates the RP item, writes the")
	fmt.Println("  ID atomically (.tmp → rename), then releases the lock. Losers poll until")
	fmt.Println("  the file appears and read the same ID. Result: exactly 1 RP item, not N.")
	fmt.Println()
	fmt.Println("TUNING:")
	fmt.Println("  RP_LOCK_RETRY_COUNT    (default 50)  — poll attempts for losing workers")
	fmt.Println("  RP_LOCK_RETRY_SLEEP_MS (default 200) — ms between polls")
	fmt.Println("  Max wait = COUNT × SLEEP_MS = 10 s at defaults.")
	fmt.Println("  Increase for 100+ workers or slow disks; defaults are fine for ≤16 workers.")
	fmt.Println()
}

func showParameterizedHelp() {
	fmt.Println()
	fmt.Println("Parameterized / Table-Driven Tests")
	fmt.Println()
	fmt.Println("  DescribeTable entries are detected automatically.")
	fmt.Println("  Parameters are extracted from Entry text using key=value patterns.")
	fmt.Println()
	fmt.Println("EXAMPLE:")
	fmt.Println(`  DescribeTable("string ops",`)
	fmt.Println(`      func(input, expected string) { ... },`)
	fmt.Println(`      Entry("input='hello' expected='HELLO'", Label(rpagent.Priority(2))),`)
	fmt.Println(`  )`)
	fmt.Println()
}

func showConfigHelp() {
	fmt.Println()
	fmt.Println("Configuration — reportportal.properties")
	fmt.Println()
	fmt.Println("REQUIRED:")
	fmt.Println("  rp.endpoint = https://your-reportportal-host")
	fmt.Println("  rp.api.key  = YOUR_API_KEY")
	fmt.Println("  rp.launch   = MY_LAUNCH_NAME")
	fmt.Println("  rp.project  = my_project")
	fmt.Println()
	fmt.Println("OPTIONAL:")
	fmt.Println("  rp.debug    = false   # set true to log HTTP traffic to rp-agent.log")
	fmt.Println()
	fmt.Println("ENVIRONMENT VARIABLES:")
	fmt.Println(
		"  RP_LOCK_RETRY_COUNT    (default 50)  — poll attempts waiting for parallel cache writes",
	)
	fmt.Println(
		"  RP_LOCK_RETRY_SLEEP_MS (default 200) — ms between polls; max wait = COUNT × SLEEP_MS = 10 s",
	)
	fmt.Println("  RP_FINALIZE_TIMEOUT    (default 3m)  — how long finishing the launch may take")
	fmt.Println("  RP_STRICT              (default false) — fail the run when reporting fails")
	fmt.Println("  RP_SKIPPED_ISSUE       (default true) — false: Skip(\"reason\") is not an issue")
	fmt.Println()
}

// finishContainers finishes Describe/Context items, deepest first (ReportPortal does not
// let an item finish before its children), each with the status and end time aggregated
// from its specs. It stops early, with ctx's error or errReportPortalNotResponding, when
// finalization is canceled or finish requests keep failing.
func finishContainers(
	ctx context.Context,
	client *gorp.ReportingClient,
	launchID string,
	containers []rpagent.ContainerResult,
	streak *failureStreak,
) error {
	if len(containers) == 0 {
		return nil
	}
	fmt.Printf("  Finishing %d container(s)...\n", len(containers))
	for _, c := range containers {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := c.End
		if end.IsZero() {
			end = time.Now()
		}
		_, err := client.FinishTest(ctx, c.ID, &openapi.FinishTestItemRQ{
			EndTime:    end,
			Status:     ptr(string(c.Status)),
			LaunchUuid: launchID,
		})
		if err != nil {
			fmt.Printf("  WARNING: failed to finish container %s: %v\n", c.ID, err)
		}
		if gerr := streak.note(err); gerr != nil {
			return gerr
		}
	}
	fmt.Printf("\n")
	return nil
}

// splitFixtureResults separates failed BeforeSuite/AfterSuite records from spec results.
func splitFixtureResults(
	all []rpagent.SharedSpecStatus,
) (specs, fixtures []rpagent.SharedSpecStatus) {
	for _, r := range all {
		if r.Fixture {
			fixtures = append(fixtures, r)
		} else {
			specs = append(specs, r)
		}
	}
	return specs, fixtures
}

// overallResult describes the run's outcome as the lines to print under "Overall Status",
// and whether the launch must be marked failed regardless of the spec results.
// specsFailed counts failed specs only; fixtureFailures counts failed BeforeSuite/AfterSuite
// nodes. hasSuiteFailures is true when ginkgo exited non-zero.
func overallResult(
	hasSuiteFailures bool,
	specsFailed, passed, skipped, fixtureFailures int,
) (lines []string, forceFailed bool) {
	switch {
	case specsFailed > 0:
		return []string{"    RESULT: TESTS FAILED"}, false
	case fixtureFailures > 0 && passed+skipped == 0:
		return []string{"    RESULT: SUITE SETUP FAILED (no tests executed)"}, true
	case fixtureFailures > 0:
		return []string{
			fmt.Sprintf(
				"    RESULT: SUITE SETUP OR TEARDOWN FAILED (%d tests ran)",
				passed+skipped,
			),
		}, true
	case !hasSuiteFailures && passed == 0 && skipped > 0:
		return []string{
			"    RESULT: ALL TESTS SKIPPED (check --focus / --label-filter — no specs matched)",
		}, false
	case !hasSuiteFailures:
		return []string{"    RESULT: ALL TESTS PASSED"}, false
	default:
		// ginkgo failed but reported nothing: a compile error or a crash before any spec.
		return []string{
			"    RESULT: SUITE SETUP FAILED (no tests executed)",
			"",
			"  Common causes:",
			"    1. Panic or error in Describe/Context bodies",
			"    2. A compile error in the test package",
			"    3. Priority() called outside an It block",
		}, true
	}
}
