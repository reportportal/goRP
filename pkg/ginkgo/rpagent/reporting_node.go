package rpagent

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"

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
	StepID         string
	ContainerIDs   []string
	ContainerNames []string
	ChildStepIDs   []string
	ChildStepNames []string
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

	InitializeAttachments()

	coordinator = NewLaunchCoordinator(sessionID)
	var err error
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
			"WARNING (PID %d): could not get suite ID for %s, staying dormant: %v",
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
		finishSpecHierarchy(spec, priority)

		wd, _ := os.Getwd()
		status := SharedSpecStatus{
			SuiteName:       getCurrentSuiteName(),
			SuiteDirectory:  wd,
			SpecName:        spec.FullText(),
			Status:          spec.State.String(),
			Priority:        priority,
			TestReferenceId: refID,
		}
		if spec.Failure.Message != "" {
			status.FailureMessage = spec.Failure.Message
			status.FailureLocation = fmt.Sprintf(
				"%s:%d",
				spec.Failure.Location.FileName,
				spec.Failure.Location.LineNumber,
			)
		}

		if agentState != nil {
			pid := os.Getpid()
			sessionID := os.Getenv("RP_SESSION_ID")
			if sessionID == "" {
				return
			}
			file := filepath.Join(
				os.TempDir(),
				fmt.Sprintf(ProcessReportFilePattern, agentState.SessionID, pid),
			)

			resultsMutex.Lock()
			defer resultsMutex.Unlock()

			f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			if err != nil {
				log.Printf("ERROR: could not open status file: %v", err)
				return
			}

			data, err := json.Marshal(status)
			if err != nil {
				_ = f.Close()
				log.Printf("ERROR: could not marshal spec status: %v", err)
				return
			}
			if _, werr := f.WriteString(string(data) + "\n"); werr != nil {
				log.Printf("ERROR: could not write spec status: %v", werr)
			}
			_ = f.Sync()
			_ = f.Close()
		}
	})

	ginkgo.ReportAfterSuite("ReportPortal Final Flush", func(report types.Report) {
		if !isInitialized || os.Getenv("RP_SESSION_ID") == "" {
			return
		}
		pid := os.Getpid()
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
						globalListener.FinishTestWithPriority(
							item.ids.ChildStepNames[i],
							gorp.Statuses.Interrupted,
							childID,
							nil,
						)
					}
					globalListener.FinishTestWithPriority(
						"interrupted",
						gorp.Statuses.Interrupted,
						item.ids.StepID,
						nil,
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
	return nil, fmt.Errorf(
		"reportportal.properties not found (searched %d levels up from %s)",
		maxDepth,
		dir,
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
	id    string
	name  string
	order int // TimelineLocation.Order of the ByStart event (monotonically increasing)
}

func finishSpecHierarchy(spec types.SpecReport, priority int) {
	if globalClient == nil || globalListener == nil {
		return
	}
	val, ok := activeHierarchies.LoadAndDelete(spec.FullText())
	if !ok {
		log.Printf("WARNING: no active hierarchy for: %s", spec.FullText())
		return
	}
	ids := val.(SpecHierarchyIDs)

	bySteps := extractAndCreateBySteps(spec, ids)

	// Rebuild ids.ChildStepIDs/Names from bySteps for finishHierarchy.
	for _, b := range bySteps {
		ids.ChildStepIDs = append(ids.ChildStepIDs, b.id)
		ids.ChildStepNames = append(ids.ChildStepNames, b.name)
	}

	processLogsWithByRouting(spec, ids.StepID, bySteps)
	processAttachmentsWithByRouting(spec, ids.StepID, bySteps)
	finishHierarchy(spec, ids, priority)
}

// extractAndCreateBySteps creates a child STEP item in RP for each By()
// statement and returns them with their timeline order for log routing.
func extractAndCreateBySteps(spec types.SpecReport, ids SpecHierarchyIDs) []byStep {
	var steps []byStep
	for _, event := range spec.SpecEvents {
		if event.SpecEventType != types.SpecEventByStart {
			continue
		}
		if event.Message == "" {
			continue
		}
		childID := globalListener.StartTestWithStats(
			event.Message,
			ids.StepID,
			nil,
			nil,
			gorp.TestItemTypes.Step,
			false,
		)
		steps = append(steps, byStep{
			id:    childID,
			name:  event.Message,
			order: event.TimelineLocation.Order,
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

// finishHierarchy closes the It step and its By() children.
// Containers (Describe/Context) are NOT finished here — they are shared across
// specs and parallel workers. The wrapper finishes them in Phase 3 via
// finishSuitesFromResults, after all workers have completed.
func finishHierarchy(spec types.SpecReport, ids SpecHierarchyIDs, priority int) {
	status := MapGinkgoStateToStatus(spec.State.String())

	if spec.Failure.Message != "" {
		msg := fmt.Sprintf("TEST FAILED\n\nLocation: %s:%d\n\nFailure:\n%s",
			spec.Failure.Location.FileName, spec.Failure.Location.LineNumber, spec.Failure.Message)
		if ids.StepID != "" {
			if err := globalListener.SendLog(ids.StepID, gorp.LogLevelError, msg); err != nil {
				log.Printf("WARNING: failed to send failure log for %s: %v", spec.LeafNodeText, err)
			}
		}
	}

	for i, childID := range ids.ChildStepIDs {
		if childID == "" {
			continue
		}
		globalListener.FinishTestWithPriority(ids.ChildStepNames[i], status, childID, nil)
	}
	if ids.StepID != "" {
		globalListener.FinishTestWithPriority(spec.LeafNodeText, status, ids.StepID, &priority)
	}
	// Containers are intentionally not finished here. See function comment.
}

func processLogsWithByRouting(spec types.SpecReport, stepID string, steps []byStep) {
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
			if err := globalListener.SendLog(target, rpLevel, parts[2]); err != nil {
				log.Printf("WARNING: failed to send log to ReportPortal: %v", err)
			}
		}
	}
}

func processAttachmentsWithByRouting(spec types.SpecReport, stepID string, steps []byStep) {
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
				)
			}
		} else if strings.HasPrefix(v, attachmentErrorPrefix) {
			raw := strings.TrimPrefix(v, attachmentErrorPrefix)
			if target != "" {
				_ = globalListener.SendLog(
					target,
					gorp.LogLevelError,
					fmt.Sprintf("Attachment error: %s", raw),
				) //nolint:errcheck
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

	gs := GinkgoSpecReport{
		ContainerHierarchyTexts:    spec.ContainerHierarchyTexts,
		ContainerHierarchyLabels:   spec.ContainerHierarchyLabels,
		LeafNodeText:               spec.LeafNodeText,
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
	suiteName := filepath.Base(wd)

	entries, err := os.ReadDir(wd)
	if err != nil {
		cachedSuiteName = suiteName
		return cachedSuiteName
	}

	re := regexp.MustCompile(`RunSpecs\s*\([^,]+,\s*"([^"]+)"`)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, "_suite_test.go") {
			continue
		}
		// Path-traversal guard
		if strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
			continue
		}
		fullPath := filepath.Join(wd, name)
		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			continue
		}
		absWd, err := filepath.Abs(wd)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(absPath, absWd) {
			continue
		}
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		if m := re.FindSubmatch(content); len(m) > 1 {
			cachedSuiteName = string(m[1])
			return cachedSuiteName
		}
	}
	cachedSuiteName = suiteName
	return cachedSuiteName
}
