package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
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

	verboseMode := hasVerboseFlag(os.Args[1:])
	_ = verboseMode

	sessionID := fmt.Sprintf("%d-%s", time.Now().Unix(), uuid.New().String())

	if err := rpagent.InitializeRPLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: could not initialise rp-agent.log: %v\n", err)
	}
	defer rpagent.CloseRPLogger()
	defer cleanup(sessionID)

	finalize := shouldFinalizeLaunch(os.Args[1:])

	if finalize {
		fmt.Printf(
			"======== Starting ReportPortal-enabled Ginkgo run | Session: %s ========\n",
			sessionID,
		)
	}

	// Run suite discovery BEFORE creating the launch so that any discovery
	// error does not leave an orphan IN_PROGRESS launch in ReportPortal.
	var (
		launchID   string
		suiteCache *rpagent.SuiteCache
	)
	if finalize {
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
		id, sc, err := createRPLaunchAndSaveState(sessionID, os.Args[1:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nERROR: Failed to create ReportPortal launch: %v\n", err)
			return 1
		}
		launchID = id
		suiteCache = sc
		_ = suiteCache

		// Only set RP_SESSION_ID after the launch (and state file) exist.
		// This prevents workers from trying to load a missing state file when
		// --dry-run or other non-test commands are used.
		_ = os.Setenv("RP_SESSION_ID", sessionID)

		fmt.Printf("\n  Launch initialization complete.\n")
		fmt.Printf("  All test processes will use this launch.\n")
		fmt.Printf("\n")
		fmt.Printf(
			"================================================================================\n",
		)
	}

	ginkgoPath, err := exec.LookPath("ginkgo")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: ginkgo not found in PATH. Install with:\n")
		fmt.Fprintf(os.Stderr, "  go install github.com/onsi/ginkgo/v2/ginkgo@latest\n")
		if finalize && launchID != "" {
			abortLaunch(sessionID, launchID)
		}
		return 1
	}

	args := buildGinkgoArgs(os.Args[1:])
	passthrough := len(os.Args) > 1 && passthroughSubcommands[os.Args[1]]

	if finalize {
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
	} else if !passthrough {
		fmt.Printf("\n  [dry-run] Command: %s %v\n\n", ginkgoPath, args)
	}

	cmd := exec.Command(ginkgoPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: failed to execute ginkgo: %v\n", err)
			exitCode = 1
		}
	}

	if finalize {
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
		fmt.Printf("\n  Step 1: Waiting for all test processes to complete...\n")

		// Workers have already exited (cmd.Run returned). The wait is only
		// needed to let the OS finish flushing NDJSON writes; cap it short.
		if err := waitForWorkersToFinish(sessionID); err != nil {
			fmt.Printf("\n  WARNING: %v\n  Continuing with available results...\n", err)
		}

		fmt.Printf("\n  Step 2: Collecting and analysing test results...\n\n")

		if err := aggregateAndFinalizeLaunch(exitCode != 0, sessionID); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: failed to aggregate and finalize launch: %v\n", err)
			return 2
		}
	}

	if finalize {
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

func main() {
	os.Exit(run())
}

// abortLaunch attempts to finish a launch as FAILED when the wrapper encounters
// a fatal error after StartLaunch has already been called. This prevents the
// launch from staying IN_PROGRESS indefinitely in ReportPortal.
func abortLaunch(sessionID, launchID string) {
	coordinator := rpagent.NewLaunchCoordinator(sessionID)
	state, err := coordinator.LoadAgentState()
	if err != nil {
		return
	}
	config, err := rpagent.LoadRPConfig()
	if err != nil {
		return
	}
	client := rpagent.NewReportingClient(state.Endpoint, state.Project, config.APIKey, config.Debug)
	_, _ = client.StopLaunch(context.Background(), launchID)
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
		fmt.Sprintf("rp_containers_%s_*.json", sessionID),
	} {
		matches, _ := filepath.Glob(filepath.Join(tempDir, pattern))
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}
	// Attachment dirs and L2 cache files include the session ID in their names
	// (set by InitializeAttachments and GetOrCreate).
	for _, pattern := range []string{
		fmt.Sprintf("rp-agent-attachments-*%s*", sessionID),
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
func waitForWorkersToFinish(sessionID string) error {
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
		time.Sleep(pollInterval)
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

// hasSkipPackageValue checks whether --skip-package already includes packageName.
func hasSkipPackageValue(args []string, packageName string) bool {
	for i, arg := range args {
		if strings.HasPrefix(arg, "--skip-package=") {
			val := strings.TrimPrefix(arg, "--skip-package=")
			for _, part := range strings.Split(val, ",") {
				if strings.TrimSpace(part) == packageName {
					return true
				}
			}
		}
		if arg == "--skip-package" && i+1 < len(args) {
			for _, part := range strings.Split(args[i+1], ",") {
				if strings.TrimSpace(part) == packageName {
					return true
				}
			}
		}
	}
	return false
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
func buildGinkgoArgs(args []string) []string {
	// Utility subcommands: pass through unchanged — they don't accept --skip-package.
	if len(args) > 0 && passthroughSubcommands[args[0]] {
		return args
	}

	// If the user already excluded rp-agent, just pass args through as-is.
	if hasSkipPackageValue(args, "rpagent") {
		return args
	}

	// Separate flags from package paths. Ginkgo requires all flags before paths.
	// We reconstruct: [flags...] [--skip-package=rpagent] [paths...]
	var flagArgs, pathArgs []string
	injected := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if strings.HasPrefix(arg, "--skip-package=") {
			val := strings.TrimPrefix(arg, "--skip-package=")
			flagArgs = append(flagArgs, "--skip-package="+val+",rpagent")
			injected = true
		} else if arg == "--skip-package" && i+1 < len(args) {
			flagArgs = append(flagArgs, "--skip-package="+args[i+1]+",rpagent")
			injected = true
			i++
		} else if isPackagePath(arg) {
			pathArgs = append(pathArgs, arg)
		} else if ginkgoValueFlags[arg] && i+1 < len(args) {
			// value flag: keep the pair together in flagArgs
			flagArgs = append(flagArgs, arg, args[i+1])
			i++
		} else {
			flagArgs = append(flagArgs, arg)
		}
		i++
	}
	if !injected {
		flagArgs = append(flagArgs, "--skip-package=rpagent")
	}
	return append(flagArgs, pathArgs...)
}

// isPackagePath reports whether arg looks like a package path rather than a flag.
// Package paths start with ./ or ../, end with /, or equal ./... or ...
// Anything starting with - is a flag, not a path.
func isPackagePath(arg string) bool {
	if strings.HasPrefix(arg, "-") {
		return false
	}
	return strings.HasPrefix(arg, "./") ||
		strings.HasPrefix(arg, "../") ||
		strings.HasSuffix(arg, "/") ||
		arg == "..." || arg == "./..."
}

// ginkgoValueFlags is the complete set of Ginkgo flags that consume the next
// token as their value when written without =. Boolean flags are NOT listed
// here. This list is used only for path extraction (parseTargetPathsAndRecursiveFlag).
var ginkgoValueFlags = map[string]bool{
	"--focus": true, "--skip": true, "--label-filter": true,
	"--procs": true, "--output-dir": true,
	"--coverprofile": true, "--covermode": true,
	"--skip-package": true, "--timeout": true, "--tags": true,
	"--seed": true, "--json-report": true, "--junit-report": true,
	"--output-interceptor-mode": true, "--poll-progress-after": true,
	"--focus-file": true, "--skip-file": true, "--flake-attempts": true,
	"--repeat": true, "--source-root": true,
	// short forms
	"-focus": true, "-skip": true, "-procs": true,
	"-skip-package": true, "-timeout": true, "-seed": true,
	"-flake-attempts": true, "-repeat": true,
}

func parseTargetPathsAndRecursiveFlag(args []string) (bool, []string) {
	hasRecursive := false
	for _, arg := range args {
		if arg == "-r" || arg == "--recursive" {
			hasRecursive = true
			break
		}
	}
	var paths []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			// Everything after -- is passed to the test binary, not ginkgo.
			break
		}
		if strings.HasPrefix(arg, "-") {
			// Flag with = value: skip the whole token.
			if strings.Contains(arg, "=") {
				i++
				continue
			}
			// Boolean flag: skip just this token.
			if !ginkgoValueFlags[arg] {
				i++
				continue
			}
			// Value-taking flag: skip this token and the next.
			i += 2
			continue
		}
		// Known Ginkgo subcommands are not package paths.
		switch arg {
		case "run",
			"build",
			"watch",
			"labels",
			"outline",
			"unfocus",
			"bootstrap",
			"generate",
			"version",
			"help":
			i++
			continue
		}
		// Handle ./... → recursive from current directory.
		if arg == "./..." {
			hasRecursive = true
			paths = append(paths, ".")
		} else {
			paths = append(paths, arg)
		}
		i++
	}
	return hasRecursive, paths
}

func createRPLaunchAndSaveState(
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
	hasRecursive, targetPaths := parseTargetPathsAndRecursiveFlag(args)
	if len(targetPaths) == 0 {
		targetPaths = []string{"."}
	}

	fmt.Printf("  Suite Discovery:\n")
	fmt.Printf("    - Targets:   %s\n", strings.Join(targetPaths, ", "))
	fmt.Printf("    - Recursive: %v\n\n", hasRecursive)
	fmt.Printf("  Scanning for test suites...\n")

	var suites []DiscoveredSuite
	for _, tp := range targetPaths {
		found, err := discoverSuites(tp, hasRecursive)
		if err != nil {
			return "", nil, fmt.Errorf("failed to discover suites in %s: %w", tp, err)
		}
		suites = append(suites, found...)
	}
	if len(suites) == 0 {
		return "", nil, fmt.Errorf("no Ginkgo suites found in %s", strings.Join(targetPaths, ", "))
	}
	fmt.Printf("  Found %d suite(s)\n\n", len(suites))

	launchRQ := &openapi.StartLaunchRQ{
		Uuid:      uuid.NewString(),
		Name:      config.LaunchName,
		StartTime: time.Now(),
		Mode:      ptr(string(gorp.LaunchModes.Default)),
	}

	fmt.Printf("  Launch Configuration:\n")
	fmt.Printf("    - Name:    %s\n", config.LaunchName)
	fmt.Printf("    - Project: %s\n", config.Project)
	fmt.Printf("\n  Creating launch...\n")

	launchRS, err := client.StartLaunch(context.Background(), launchRQ)
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

	// From this point on, any error must finish/stop the launch to prevent orphans.
	defer func() {
		if retErr != nil && launchID != "" {
			_, _ = client.StopLaunch(
				context.Background(),
				launchID,
			)
		}
	}()

	sc := &rpagent.SuiteCache{Suites: make(map[string]string)}

	fmt.Printf("  Creating suites in ReportPortal:\n\n")
	for i, suite := range suites {
		suiteRS, err := client.StartTest(context.Background(), &openapi.StartTestItemRQ{
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

func aggregateAndFinalizeLaunch(hasSuiteFailures bool, sessionID string) error {
	if sessionID == "" {
		sessionID = os.Getenv("RP_SESSION_ID")
	}
	if sessionID == "" {
		return fmt.Errorf("RP_SESSION_ID not set")
	}

	coordinator := rpagent.NewLaunchCoordinator(sessionID)

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
	fmt.Printf("  Total Tests Executed: %d\n\n", len(allStatuses))

	passed, failed, interrupted, skipped := 0, 0, 0, 0
	priorityStats := make(map[int]map[string]int)

	for _, status := range allStatuses {
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
				"WARNING: unrecognised status %q for %s\n",
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
		for _, status := range allStatuses {
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

	fmt.Printf("  Overall Status:\n")
	if !hasSuiteFailures && totalFailed == 0 && passed == 0 && skipped > 0 {
		fmt.Printf(
			"    RESULT: ALL TESTS SKIPPED (check --focus / --label-filter — no specs matched)\n\n",
		)
	} else if !hasSuiteFailures && totalFailed == 0 {
		fmt.Printf("    RESULT: ALL TESTS PASSED\n\n")
	} else if hasSuiteFailures && totalFailed == 0 {
		fmt.Printf("    RESULT: SUITE SETUP FAILED (no tests executed)\n\n")
		fmt.Printf("  Common causes:\n")
		fmt.Printf("    1. Panic or error in Describe/Context bodies\n")
		fmt.Printf("    2. BeforeSuite / AfterSuite failures\n")
		fmt.Printf("    3. Priority() called outside an It block\n\n")
		totalFailed = 1 // force launch to be marked failed
	} else {
		fmt.Printf("    RESULT: TESTS FAILED\n\n")
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

	if err := finishSuitesFromResults(sessionID, allStatuses, hasSuiteFailures); err != nil {
		return fmt.Errorf("failed to finish suites: %w", err)
	}

	if err := rpagent.FinalizeLaunch(hasSuiteFailures || totalFailed > 0); err != nil {
		return fmt.Errorf("failed to finalize launch: %w", err)
	}

	return nil
}

func finishSuitesFromResults(
	sessionID string,
	allStatuses []rpagent.SharedSpecStatus,
	hasSuiteFailures bool,
) error {
	suiteCache, err := rpagent.LoadSuiteCache(sessionID)
	if err != nil {
		return fmt.Errorf("failed to load suite cache: %w", err)
	}

	coordinator := rpagent.NewLaunchCoordinator(sessionID)
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
	fmt.Printf("  Finishing %d suite(s)...\n\n", len(suiteCache.Suites))

	for suiteDir, suiteID := range suiteCache.Suites {
		results := suiteResults[suiteDir]
		suiteStatus := gorp.Statuses.Passed

		if len(results) == 0 {
			// A suite with zero results could be:
			//   a) a compile error / crash (hasSuiteFailures) → mark FAILED
			//   b) all specs were filtered by --focus → mark PASSED (not a failure)
			if hasSuiteFailures {
				suiteStatus = gorp.Statuses.Failed
			}
			fmt.Printf("  Suite: %s — %s (0 tests)\n", filepath.Base(suiteDir), suiteStatus)
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
			fmt.Printf(
				"  Suite: %s — %s (%d tests)\n",
				filepath.Base(suiteDir),
				suiteStatus,
				len(results),
			)
		}

		_, err := client.FinishTest(context.Background(), suiteID, &openapi.FinishTestItemRQ{
			EndTime:    time.Now(),
			Status:     ptr(string(suiteStatus)),
			LaunchUuid: agentState.LaunchID,
		})
		if err != nil {
			fmt.Printf("  WARNING: failed to finish suite %s: %v\n", suiteID, err)
		}
	}
	fmt.Printf("\n")
	return nil
}

func hasVerboseFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-v" || arg == "--verbose" || arg == "-vv" {
			return true
		}
	}
	return false
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
		"halp":  "Did you mean: ginkgo-rp help",
		// version variants
		"--v":       "Did you mean: ginkgo-rp --version  (or -v for verbose ginkgo output)",
		"--ver":     "Did you mean: ginkgo-rp --version",
		"--versoin": "Did you mean: ginkgo-rp --version",
		"verison":   "Did you mean: ginkgo-rp version",
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
	fmt.Println("  - Default priority is P1 if not specified.")
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
	fmt.Println()
}
