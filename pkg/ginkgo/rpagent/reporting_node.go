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

// init eagerly initialises the agent in every Ginkgo worker process.
// If RP_SESSION_ID is not set the agent is inactive (plain Ginkgo mode).
func init() {
	sessionID := os.Getenv("RP_SESSION_ID")
	if sessionID == "" {
		return
	}

	pid := os.Getpid()
	LogVerboseOperation(fmt.Sprintf("initialising agent (PID %d)", pid))

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
		log.Printf("WARNING: could not initialise rp-agent.log: %v", initErr)
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

// setupExitHandlers installs a signal handler that marks in-flight RP items as INTERRUPTED.
// It does NOT call os.Exit — Ginkgo's own signal handling runs AfterEach/AfterSuite normally,
// and those hooks deliver interrupted specs to ReportAfterEach with State=interrupted.
// We only need to write the done-flag and close the logger here.
func setupExitHandlers() {
	exitHandlerOnce.Do(func() {
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

			// Finish any items that are still open. We do this before Ginkgo's
			// own AfterEach runs so RP does not show "In Progress" items if the
			// process is killed rather than interrupted gracefully.
			//
			// We snapshot the map first, then delete entries, to avoid a race
			// with the concurrent ReportAfterEach path.
			var snapshot []struct {
				key string
				ids SpecHierarchyIDs
			}
			activeHierarchies.Range(func(key, val any) bool {
				ids, ok := val.(SpecHierarchyIDs)
				if ok {
					snapshot = append(snapshot, struct {
						key string
						ids SpecHierarchyIDs
					}{key.(string), ids})
				}
				return true
			})
			for _, item := range snapshot {
				activeHierarchies.Delete(item.key)
				if globalListener != nil {
					for i, childID := range item.ids.ChildStepIDs {
						globalListener.FinishTest(
							item.ids.ChildStepNames[i],
							gorp.Statuses.Interrupted,
							childID,
							time.Now(),
						)
					}
					globalListener.FinishTest(
						"interrupted",
						gorp.Statuses.Interrupted,
						item.ids.StepID,
						time.Now(),
					)
					// Do NOT finish containers here — they are shared across specs;
					// the wrapper finishes them in Phase 3.
				}
			}

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
			// Do NOT call os.Exit here. Let Ginkgo handle the interrupt.
		}()
	})
}

// LoadRPConfig walks up from cwd looking for reportportal.properties.
// The search depth defaults to 10 levels and can be overridden by setting
// the RP_PROPERTIES_DEPTH environment variable to any positive integer.
func LoadRPConfig() (*Config, error) {
	maxDepth := 10
	if v := os.Getenv("RP_PROPERTIES_DEPTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxDepth = n
		}
	}

	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for i := 0; i < maxDepth; i++ {
		p := filepath.Join(dir, "reportportal.properties")
		if _, err := os.Stat(p); err == nil {
			return LoadConfig(p)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// No file (typical in CI): the environment alone may be enough.
	if cfg, envErr := ConfigFromEnv(); envErr == nil {
		return cfg, nil
	}
	return nil, fmt.Errorf(
		"reportportal.properties not found (searched %d levels up) and RP_ENDPOINT/RP_PROJECT/RP_API_KEY are not all set",
		maxDepth,
	)
}

func startSpecHierarchy(
	spec types.SpecReport,
	parameters []*Parameter,
	priority int,
	refID string,
) SpecHierarchyIDs {
	if globalClient == nil || globalLaunchID == "" || globalSuiteID == "" || coordinator == nil {
		log.Printf("CRITICAL: global agent state not initialised")
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
			if err := globalListener.SendLog(
				failureTarget,
				level,
				msg,
				failedAt,
			); err != nil {
				log.Printf("WARNING: failed to send failure log for %s: %v", spec.LeafNodeText, err)
			}
		}
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
			if err := globalListener.SendLog(
				target,
				rpLevel,
				parts[2],
				win.clamp(entry.Time),
			); err != nil {
				log.Printf("WARNING: failed to send log to ReportPortal: %v", err)
			}
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
				_ = globalListener.SendLog(
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
	if _, name, ok := FindSuite(wd); ok {
		cachedSuiteName = name
	} else {
		cachedSuiteName = filepath.Base(wd)
	}
	return cachedSuiteName
}
