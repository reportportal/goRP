package rpagent

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"

	"github.com/reportportal/goRP/v5/pkg/gorp"
)

// Global state for a single Ginkgo worker process.
var (
	globalClient   *gorp.ReportingClient
	globalListener *TestListener
	globalLaunchID string
	globalSuiteID  string
	coordinator    *LaunchCoordinator
	agentState     *AgentState

	resultsMutex sync.Mutex

	cachedSuiteName string

	// activeHierarchies correlates ReportBeforeEach → ReportAfterEach by spec full-text.
	activeHierarchies sync.Map

	isInitialized   bool
	exitHandlerOnce sync.Once
)

// SpecHierarchyIDs holds the ReportPortal IDs created for one spec's hierarchy.
type SpecHierarchyIDs struct {
	Template  StartItem // how the spec item was started; reused to start retry attempts
	StepID    string
	Identity  ItemIdentity // run-independent identity of the spec item
	StartTime time.Time    // when the spec item was started in ReportPortal

	ContainerIDs   []string
	ContainerNames []string
	ChildStepIDs   []string
	ChildStepNames []string
	ChildStepEnds  []time.Time

	ChildStepStatuses []gorp.Status
}

// init eagerly initializes the agent in every Ginkgo worker process.
// If RP_SESSION_ID is not set the agent is inactive (plain Ginkgo mode).
func init() {
	sessionID := os.Getenv("RP_SESSION_ID")
	if sessionID == "" {
		return
	}

	pid := os.Getpid()
	LogVerboseOperation(fmt.Sprintf("initializing agent (PID %d)", pid))

	var err error
	coordinator, err = NewLaunchCoordinator(sessionID)
	if err != nil {
		log.Printf("WARNING (PID %d): %v, staying dormant", pid, err)
		return
	}
	agentState, err = coordinator.LoadAgentState()
	if err != nil {
		// State file missing means the wrapper did not write it (e.g. --dry-run).
		// Do not fatal — stay dormant instead.
		log.Printf("WARNING (PID %d): could not load agent state, staying dormant: %v", pid, err)
		return
	}

	globalLaunchID = agentState.LaunchID

	// API key is not stored in the state file (security: avoids plaintext in $TMPDIR).
	// Re-read it from reportportal.properties.
	config, configErr := LoadRPConfig()
	if configErr != nil {
		log.Printf(
			"WARNING (PID %d): could not load reportportal.properties, staying dormant: %v",
			pid,
			configErr,
		)
		return
	}

	if initErr := InitializeRPLogger(); initErr != nil {
		log.Printf("WARNING: could not initialize rp-agent.log: %v", initErr)
	}

	globalClient = NewReportingClient(
		agentState.Endpoint,
		agentState.Project,
		config.APIKey,
		config.Debug,
	)

	globalListener = NewTestListener(globalClient, globalLaunchID, agentState.LaunchName)

	// Resolve symlinks so the key always matches what the wrapper stored.
	wd, err := os.Getwd()
	if err != nil {
		log.Printf(
			"WARNING (PID %d): failed to get working directory, staying dormant: %v",
			pid,
			err,
		)
		return
	}
	resolvedWd, err := filepath.EvalSymlinks(wd)
	if err != nil {
		resolvedWd = wd // best-effort
	}
	suiteID, suiteErr := GetSuiteIDForDirectory(sessionID, resolvedWd)
	if suiteErr != nil {
		log.Printf(
			"ERROR (PID %d): this package (%s) was not registered by ginkgo-rp, so its specs "+
				"will NOT be reported to ReportPortal (tests still run): %v",
			pid,
			resolvedWd,
			suiteErr,
		)
		return
	}
	globalSuiteID = suiteID

	activeHierarchies = sync.Map{}
	setupExitHandlers()
	isInitialized = true
	LogVerboseOperation(
		fmt.Sprintf(
			"agent ready (launch: %s, suite: %s, PID: %d)",
			globalLaunchID,
			globalSuiteID,
			pid,
		),
	)

	// ── Ginkgo reporting hooks ──────────────────────────────────────────────

	ginkgo.ReportBeforeEach(func(spec types.SpecReport) {
		if !isInitialized || os.Getenv("RP_SESSION_ID") == "" {
			return
		}

		priority := ExtractPriorityFromReportEntry(spec)
		refID := ExtractTestReferenceIdFromReportEntry(spec)

		var parameters []*Parameter
		if IsDescribeTableEntry(spec) {
			parameters = ExtractParametersFromEntryName(spec.LeafNodeText)
		}

		ids := startSpecHierarchy(spec, parameters, priority, refID)
		if ids.StepID != "" {
			activeHierarchies.Store(spec.FullText(), ids)
		}
	})

	ginkgo.JustAfterEach(func() {
		if isInitialized && os.Getenv("RP_SESSION_ID") != "" {
			recordAttemptFailure(ginkgo.CurrentSpecReport())
		}
	})

	ginkgo.ReportAfterEach(func(spec types.SpecReport) {
		if !isInitialized || os.Getenv("RP_SESSION_ID") == "" {
			return
		}

		priority := ExtractPriorityFromReportEntry(spec)
		refID := ExtractTestReferenceIdFromReportEntry(spec)
		ids := finishSpecHierarchy(spec)

		wd, _ := os.Getwd()
		status := SharedSpecStatus{
			SuiteName:       getCurrentSuiteName(),
			SuiteDirectory:  wd,
			SpecName:        spec.FullText(),
			Status:          spec.State.String(),
			Priority:        priority,
			TestReferenceId: refID,
			ContainerIDs:    ids.ContainerIDs,
			EndTime:         spec.EndTime,
		}
		if spec.Failure.Message != "" {
			status.FailureMessage = spec.Failure.Message
			status.FailureLocation = fmt.Sprintf(
				"%s:%d",
				spec.Failure.Location.FileName,
				spec.Failure.Location.LineNumber,
			)
		}

		appendSpecStatus(status)
	})

	ginkgo.ReportAfterSuite("ReportPortal Final Flush", func(report types.Report) {
		if !isInitialized || os.Getenv("RP_SESSION_ID") == "" {
			return
		}
		pid := os.Getpid()
		reportSuiteFixtureFailures(report)
		finishOpenHierarchies()
		CleanupAttachments()
		if agentState != nil {
			flagPath := filepath.Join(
				os.TempDir(),
				fmt.Sprintf("rp_done_%s_%d.flag", agentState.SessionID, pid),
			)
			if f, err := os.Create(flagPath); err == nil {
				_ = f.Close()
			}
		}
		CloseRPLogger()
	})
}

// setupExitHandlers installs a signal handler that only writes the done-flag and closes
// the logger. It does NOT finish items and does NOT call os.Exit: Ginkgo's own interrupt
// handling delivers the running spec to ReportAfterEach (State=interrupted), which needs
// the spec's data intact in activeHierarchies. Anything still open after the suite is
// finished by finishOpenHierarchies in ReportAfterSuite.
func setupExitHandlers() {
	exitHandlerOnce.Do(func() {
		state := agentState // read now: the goroutine below must not touch the global
		ch := make(chan os.Signal, 1)
		// Do NOT catch SIGQUIT — that delivers Go's goroutine dump.
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		go func() {
			sig := <-ch
			pid := os.Getpid()
			LogVerboseOperation(
				fmt.Sprintf(
					"received signal %v (PID %d), deferring to Ginkgo interrupt handling",
					sig,
					pid,
				),
			)

			if state != nil {
				flagPath := filepath.Join(
					os.TempDir(),
					fmt.Sprintf("rp_done_%s_%d.flag", state.SessionID, pid),
				)
				if f, err := os.Create(flagPath); err == nil {
					_ = f.Close()
				}
			}
			CloseRPLogger()
			// Do NOT call os.Exit here. Let Ginkgo handle the interrupt.
		}()
	})
}

// ConfigFileEnv names the environment variable through which the wrapper tells workers
// which properties file it used. Workers run in their own package directory, which may be
// outside the wrapper's tree, so searching upwards from there would not find it.
const ConfigFileEnv = "RP_CONFIG_FILE"

// ConfigFilePath finds the properties file: the path in RP_CONFIG_FILE if set and present,
// otherwise the nearest reportportal.properties walking up from the working directory.
// The search depth defaults to 10 levels and can be overridden with RP_PROPERTIES_DEPTH.
func ConfigFilePath() (path string, searchedLevels int, ok bool) {
	if p := os.Getenv(ConfigFileEnv); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, 0, true
		}
		log.Printf("WARNING: %s=%q does not exist, searching upwards instead", ConfigFileEnv, p)
	}
	searchedLevels = 10
	if v := os.Getenv("RP_PROPERTIES_DEPTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			searchedLevels = n
		}
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", searchedLevels, false
	}
	for i := 0; i < searchedLevels; i++ {
		p := filepath.Join(dir, "reportportal.properties")
		if _, err := os.Stat(p); err == nil {
			return p, searchedLevels, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", searchedLevels, false
}

// LoadRPConfig loads the configuration from the properties file found by ConfigFilePath,
// with environment variables taking precedence, or from the environment alone.
func LoadRPConfig() (*Config, error) {
	path, levels, ok := ConfigFilePath()
	if ok {
		return LoadConfig(path)
	}
	// No file (typical in CI): the environment alone may be enough.
	if cfg, envErr := ConfigFromEnv(); envErr == nil {
		return cfg, nil
	}
	return nil, fmt.Errorf(
		"reportportal.properties not found (searched %d levels up) and RP_ENDPOINT/RP_PROJECT/RP_API_KEY are not all set",
		levels,
	)
}

func startSpecHierarchy(
	spec types.SpecReport,
	parameters []*Parameter,
	priority int,
	refID string,
) SpecHierarchyIDs {
	if globalClient == nil || globalLaunchID == "" || globalSuiteID == "" || coordinator == nil {
		log.Printf("CRITICAL: global agent state not initialized")
		return SpecHierarchyIDs{}
	}
	ccm := coordinator.NewContainerCacheManager()
	gs := convertToGinkgoSpec(spec)
	gs.Priority = &priority
	gs.TestReferenceId = refID
	return ProcessSpecWithHierarchyAndReturnIDs(
		globalClient,
		globalListener,
		gs,
		globalSuiteID,
		ccm,
		parameters,
	)
}

// byStep pairs a By() child item ID with its timeline order so logs can be
// routed to the correct child based on when they were emitted.
type byStep struct {
	id     string
	name   string
	order  int // TimelineLocation.Order of the ByStart event (monotonically increasing)
	end    time.Time
	status gorp.Status
}

func finishSpecHierarchy(spec types.SpecReport) SpecHierarchyIDs {
	if globalClient == nil || globalListener == nil {
		return SpecHierarchyIDs{}
	}
	val, ok := activeHierarchies.LoadAndDelete(spec.FullText())
	if !ok {
		log.Printf("WARNING: no active hierarchy for: %s", spec.FullText())
		return SpecHierarchyIDs{}
	}
	ids := val.(SpecHierarchyIDs)

	// A spec Ginkgo ran several times (FlakeAttempts, MustPassRepeatedly) becomes one RP
	// item per attempt, each a retry of the previous one.
	attempts := splitAttempts(spec, takeAttemptFailures(spec))
	if len(attempts) == 1 {
		return reportAttempt(spec, ids)
	}
	var final SpecHierarchyIDs
	prevID := ""
	for k, attempt := range attempts {
		cur := ids
		if k > 0 {
			item := ids.Template
			item.StartTime = attempt.StartTime
			item.RetryOf = prevID
			cur.StepID = globalListener.Start(item)
			cur.StartTime = attempt.StartTime
			cur.ChildStepIDs, cur.ChildStepNames, cur.ChildStepEnds, cur.ChildStepStatuses = nil, nil, nil, nil
		}
		if cur.StepID != "" {
			note := fmt.Sprintf("Attempt %d of %d", k+1, len(attempts))
			globalListener.QueueLog(cur.StepID, gorp.LogLevelInfo, note, cur.StartTime)
		}
		final = reportAttempt(attempt, cur)
		prevID = final.StepID
	}
	return final
}

// reportAttempt reports one run of a spec onto the already started item ids.StepID.
func reportAttempt(spec types.SpecReport, ids SpecHierarchyIDs) SpecHierarchyIDs {
	// Everything below is stamped with the times Ginkgo recorded while the spec ran,
	// clamped into the spec item's own window, not with the time we upload it.
	win := specWindow(ids.StartTime, spec.EndTime)
	plans := planBySteps(spec.SpecEvents, win)
	bySteps := extractAndCreateBySteps(spec, ids, win, plans)

	// Rebuild ids.ChildStep* from bySteps for finishHierarchy.
	for _, b := range bySteps {
		ids.ChildStepIDs = append(ids.ChildStepIDs, b.id)
		ids.ChildStepNames = append(ids.ChildStepNames, b.name)
		ids.ChildStepEnds = append(ids.ChildStepEnds, b.end)
		ids.ChildStepStatuses = append(ids.ChildStepStatuses, b.status)
	}

	processLogsWithByRouting(spec, ids.StepID, bySteps, win)
	processCapturedOutput(spec, ids.StepID, bySteps, plans, win)
	processAttachmentsWithByRouting(spec, ids.StepID, bySteps, win)

	// The failure message belongs on the step that was running when it happened, so it
	// shows right under the red step; with no step running it stays on the spec item.
	failureTarget, failureWin := ids.StepID, win
	order, hasFailure := failureOrder(spec)
	if i := runningStep(plans, order, hasFailure); i >= 0 && bySteps[i].id != "" {
		failureTarget = bySteps[i].id
		failureWin = timeWindow{start: plans[i].start, end: plans[i].end}
	}
	finishHierarchy(spec, ids, win, failureTarget, failureWin)
	return ids
}

// extractAndCreateBySteps creates a child STEP item in RP for each By()
// statement and returns them with their timeline order for log routing.
// failureOrder returns the timeline order at which the spec failed, and whether the spec
// recorded a failure at all (a failed assertion, a skip, an interrupt).
func failureOrder(spec types.SpecReport) (order int, ok bool) {
	return spec.Failure.TimelineLocation.Order,
		spec.Failure.Message != "" || spec.Failure.TimelineLocation.Order != 0
}

func extractAndCreateBySteps(
	spec types.SpecReport,
	ids SpecHierarchyIDs,
	win timeWindow,
	plans []stepPlan,
) []byStep {
	order, hasFailure := failureOrder(spec)
	statuses := stepStatuses(plans, MapGinkgoStateToStatus(spec.State.String()), order, hasFailure)
	steps := make([]byStep, 0, len(plans))
	occurrences := make(map[string]int)
	for i, plan := range plans {
		occurrences[plan.name]++
		childID := globalListener.Start(StartItem{
			Name:      plan.name,
			ParentID:  ids.StepID,
			Type:      gorp.TestItemTypes.Step,
			NoStats:   true,
			StartTime: plan.start,
			Identity:  stepIdentity(ids.Identity, plan.name, occurrences[plan.name]),
		})
		steps = append(steps, byStep{
			id:     childID,
			name:   plan.name,
			order:  plan.order,
			end:    plan.end,
			status: statuses[i],
		})
	}
	return steps
}

// targetIDForOrder returns the By() child ID that owns the given timeline
// order, or the It item's stepID if no By() precedes it.
// steps must be sorted ascending by order (Ginkgo guarantees this).
func targetIDForOrder(order int, fallback string, steps []byStep) string {
	target := fallback
	for _, b := range steps {
		if b.order <= order {
			target = b.id
		}
	}
	return target
}

// failureLog builds the log for the message Ginkgo recorded when a spec did not pass.
// Ginkgo stores Skip("reason") as a failure internally, but a skip is not an error: it is
// reported as an informational "SKIPPED" entry, not as an ERROR "TEST FAILED".
func failureLog(spec types.SpecReport, status gorp.Status) (level, msg string) {
	where := fmt.Sprintf("%s:%d", spec.Failure.Location.FileName, spec.Failure.Location.LineNumber)
	if status == gorp.Statuses.Skipped {
		return gorp.LogLevelInfo, fmt.Sprintf(
			"SKIPPED\n\nLocation: %s\n\nReason:\n%s",
			where,
			spec.Failure.Message,
		)
	}
	return gorp.LogLevelError, fmt.Sprintf(
		"TEST FAILED\n\nLocation: %s\n\nFailure:\n%s",
		where,
		spec.Failure.Message,
	)
}

// finishHierarchy closes the It step and its By() children.
// Containers (Describe/Context) are NOT finished here — they are shared across
// specs and parallel workers. The wrapper finishes them in Phase 3 via
// finishSuitesFromResults, after all workers have completed.
func finishHierarchy(
	spec types.SpecReport,
	ids SpecHierarchyIDs,
	win timeWindow,
	failureTarget string,
	failureWin timeWindow,
) {
	status := MapGinkgoStateToStatus(spec.State.String())

	if spec.Failure.Message != "" {
		level, msg := failureLog(spec, status)
		if failureTarget != "" {
			failedAt := failureWin.clamp(spec.Failure.TimelineLocation.Time)
			globalListener.QueueLog(failureTarget, level, msg, failedAt)
		}
	}
	// Every log of this spec goes out in one batch, before its items are finished.
	if err := globalListener.FlushLogs(); err != nil {
		log.Printf("WARNING: failed to send logs for %s: %v", spec.LeafNodeText, err)
	}

	for i, childID := range ids.ChildStepIDs {
		if childID == "" {
			continue
		}
		globalListener.FinishTest(
			ids.ChildStepNames[i],
			ids.ChildStepStatuses[i],
			childID,
			ids.ChildStepEnds[i],
		)
	}
	if ids.StepID != "" {
		globalListener.FinishTest(spec.LeafNodeText, status, ids.StepID, win.end)
	}
	// Containers are intentionally not finished here. See function comment.
}

func processLogsWithByRouting(
	spec types.SpecReport,
	stepID string,
	steps []byStep,
	win timeWindow,
) {
	for _, entry := range spec.ReportEntries {
		// Log() calls ginkgo.AddReportEntry(name) with no extra args, so the payload
		// is in entry.Name, not entry.Value.
		v := entry.Name
		if !strings.HasPrefix(v, "RP_LOG#") {
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(v, "RP_LOG#"), "#", 3)
		if len(parts) != 3 {
			continue
		}
		level := strings.ToUpper(parts[1])
		var rpLevel string
		switch level {
		case "DEBUG":
			rpLevel = gorp.LogLevelDebug
		case "INFO":
			rpLevel = gorp.LogLevelInfo
		case "WARN", "WARNING":
			rpLevel = logLevelWarn
		case "ERROR":
			rpLevel = gorp.LogLevelError
		case "FATAL":
			rpLevel = logLevelFatal
		default:
			rpLevel = gorp.LogLevelInfo
		}
		target := targetIDForOrder(entry.TimelineLocation.Order, stepID, steps)
		if target == "" {
			target = stepID // fall back to It step if By() child start failed
		}
		if target != "" {
			globalListener.QueueLog(target, rpLevel, parts[2], win.clamp(entry.Time))
		}
	}
}

func processAttachmentsWithByRouting(
	spec types.SpecReport,
	stepID string,
	steps []byStep,
	win timeWindow,
) {
	for _, entry := range spec.ReportEntries {
		v := fmt.Sprintf("%v", entry.Value)
		target := targetIDForOrder(entry.TimelineLocation.Order, stepID, steps)
		if target == "" {
			target = stepID // fall back to It step if By() child start failed
		}
		if strings.HasPrefix(v, attachmentPrefix) {
			tmpPath, origName, message, err := ParseAttachmentPayload(v)
			if err != nil {
				log.Printf("WARNING: failed to parse attachment payload: %v", err)
				continue
			}
			if target != "" {
				_ = globalListener.SendAttachmentLog(
					target,
					gorp.LogLevelInfo,
					message,
					tmpPath,
					origName,
					win.clamp(entry.Time),
				)
			}
		} else if strings.HasPrefix(v, attachmentErrorPrefix) {
			raw := strings.TrimPrefix(v, attachmentErrorPrefix)
			if target != "" {
				globalListener.QueueLog(
					target,
					gorp.LogLevelError,
					fmt.Sprintf("Attachment error: %s", raw),
					win.clamp(entry.Time),
				)
			}
		}
	}
}

func convertToGinkgoSpec(spec types.SpecReport) GinkgoSpecReport {
	priority := ExtractPriorityFromReportEntry(spec)
	refID := ExtractTestReferenceIdFromReportEntry(spec)

	var events []SpecEvent
	for _, e := range spec.SpecEvents {
		events = append(
			events,
			SpecEvent{SpecEventType: e.SpecEventType.String(), Message: e.Message},
		)
	}

	containerLocs := make([]CodeLoc, 0, len(spec.ContainerHierarchyLocations))
	for _, l := range spec.ContainerHierarchyLocations {
		containerLocs = append(
			containerLocs,
			CodeLoc{FileName: l.FileName, LineNumber: l.LineNumber},
		)
	}

	gs := GinkgoSpecReport{
		ContainerHierarchyTexts:  spec.ContainerHierarchyTexts,
		ContainerHierarchyLabels: spec.ContainerHierarchyLabels,
		ContainerHierarchyLocs:   containerLocs,
		LeafNodeText:             spec.LeafNodeText,
		LeafNodeLoc: CodeLoc{
			FileName:   spec.LeafNodeLocation.FileName,
			LineNumber: spec.LeafNodeLocation.LineNumber,
		},
		FullText:                   spec.FullText(),
		State:                      spec.State.String(),
		CapturedStdOutErr:          spec.CapturedStdOutErr,
		CapturedGinkgoWriterOutput: spec.CapturedGinkgoWriterOutput,
		Labels:                     spec.Labels(),
		Priority:                   &priority,
		TestReferenceId:            refID,
		SpecEvents:                 events,
	}
	if spec.Failure.Message != "" {
		gs.Failure = &struct {
			Message  string `json:"Message"`
			Location struct {
				FileName   string `json:"FileName"`
				LineNumber int    `json:"LineNumber"`
			} `json:"Location"`
		}{
			Message: spec.Failure.Message,
			Location: struct {
				FileName   string `json:"FileName"`
				LineNumber int    `json:"LineNumber"`
			}{FileName: spec.Failure.Location.FileName, LineNumber: spec.Failure.Location.LineNumber},
		}
	}
	return gs
}

func getCurrentSuiteName() string {
	if cachedSuiteName != "" {
		return cachedSuiteName
	}
	wd, err := os.Getwd()
	if err != nil {
		cachedSuiteName = "Unknown Suite"
		return cachedSuiteName
	}
	if _, name, ok := FindSuite(wd, BuildTagsOfThisBinary()); ok {
		cachedSuiteName = name
	} else {
		cachedSuiteName = filepath.Base(wd)
	}
	return cachedSuiteName
}

// finishOpenHierarchies finishes every spec item that never reached ReportAfterEach
// (for example when the process is interrupted before Ginkgo reports the spec), so RP
// does not keep them In Progress.
func finishOpenHierarchies() {
	activeHierarchies.Range(func(key, val any) bool {
		activeHierarchies.Delete(key)
		ids, ok := val.(SpecHierarchyIDs)
		if !ok || globalListener == nil {
			return true
		}
		for i, childID := range ids.ChildStepIDs {
			globalListener.FinishTest(
				ids.ChildStepNames[i], gorp.Statuses.Interrupted, childID, time.Now(),
			)
		}
		globalListener.FinishTest("interrupted", gorp.Statuses.Interrupted, ids.StepID, time.Now())
		return true
	})
}
