package rpagent

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/onsi/ginkgo/v2/types"

	"github.com/reportportal/goRP/v5/pkg/gorp"
)

const (
	suiteSetupNodes    = types.NodeTypeBeforeSuite | types.NodeTypeSynchronizedBeforeSuite
	suiteTeardownNodes = types.NodeTypeAfterSuite | types.NodeTypeSynchronizedAfterSuite
)

// fixtureFailure is a failed suite-level node: BeforeSuite, AfterSuite or their
// Synchronized variants.
type fixtureFailure struct {
	name     string // "[BeforeSuite]", as Ginkgo prints it
	teardown bool   // an AfterSuite rather than a BeforeSuite
	state    string // Ginkgo's state: failed, panicked, interrupted, ...
	message  string
	failedAt CodeLoc // where the assertion failed
	node     CodeLoc // where the node is declared
	start    time.Time
	end      time.Time
}

// suiteFixtureFailures extracts the failed suite-level nodes from a suite's spec reports.
//
// Suite-level nodes are not specs, so they never reach ReportAfterEach; they appear only
// in the report handed to ReportAfterSuite. That report merges every parallel process, and
// a plain BeforeSuite/AfterSuite runs on each process, so one failure arrives once per
// process. Reports are deduplicated by node type and location. When several processes
// fail a SynchronizedBeforeSuite, the first report is the real failure (process 1's) and
// later ones only say "failed on Ginkgo parallel process #1", so the first one wins.
func suiteFixtureFailures(reports types.SpecReports) []fixtureFailure {
	seen := make(map[string]bool)
	var failures []fixtureFailure
	for _, r := range reports {
		if !r.LeafNodeType.Is(suiteSetupNodes|suiteTeardownNodes) ||
			!isFailureState(r.State.String()) {
			continue
		}
		key := fmt.Sprintf(
			"%v|%s:%d",
			r.LeafNodeType,
			r.LeafNodeLocation.FileName,
			r.LeafNodeLocation.LineNumber,
		)
		if seen[key] {
			continue
		}
		seen[key] = true
		failures = append(failures, fixtureFailure{
			name:     "[" + r.LeafNodeType.String() + "]",
			teardown: r.LeafNodeType.Is(suiteTeardownNodes),
			state:    r.State.String(),
			message:  stripANSI(r.Failure.Message),
			failedAt: CodeLoc{
				FileName:   r.Failure.Location.FileName,
				LineNumber: r.Failure.Location.LineNumber,
			},
			node: CodeLoc{
				FileName:   r.LeafNodeLocation.FileName,
				LineNumber: r.LeafNodeLocation.LineNumber,
			},
			start: r.StartTime,
			end:   r.EndTime,
		})
	}
	return failures
}

// reportSuiteFixtureFailures reports each failed suite-level node as its own item under the
// suite, with the failure as an ERROR log, and records it for the wrapper so the suite and
// the launch end up FAILED. Before this, such a failure only changed the exit code: no item,
// no log, and a suite whose specs all passed was reported PASSED.
func reportSuiteFixtureFailures(report types.Report) {
	if globalListener == nil || globalSuiteID == "" {
		return
	}
	notRun := specsNotRun(report)
	for _, f := range suiteFixtureFailures(report.SpecReports) {
		reportFixtureFailure(f, notRun)
	}
}

// specsNotRun returns how many specs Ginkgo meant to run but did not. When a BeforeSuite
// fails Ginkgo skips every spec and reports none of them, so the suite would otherwise look
// empty for no visible reason. Ginkgo knows how many specs there were, not their names.
func specsNotRun(report types.Report) int {
	ran := 0
	for _, r := range report.SpecReports {
		if !r.LeafNodeType.Is(types.NodeTypesForSuiteLevelNodes) {
			ran++
		}
	}
	return max(report.PreRunStats.SpecsThatWillRun-ran, 0)
}

// notRunNote is the sentence added to a failed setup fixture's log, or "" when no spec is affected.
func notRunNote(f fixtureFailure, notRun int) string {
	if f.teardown || notRun <= 0 {
		return ""
	}
	if notRun == 1 {
		return fmt.Sprintf("\n\n1 spec was not run because %s failed.", f.name)
	}
	return fmt.Sprintf("\n\n%d specs were not run because %s failed.", notRun, f.name)
}

func reportFixtureFailure(f fixtureFailure, notRun int) {
	status := MapGinkgoStateToStatus(f.state)
	start, end := orNow(f.start), orNow(f.end)
	if end.Before(start) {
		end = start
	}

	itemType := gorp.TestItemTypes.BeforeSuite
	if f.teardown {
		itemType = gorp.TestItemTypes.AfterSuite
	}
	where := f.failedAt
	if where.FileName == "" {
		where = f.node
	}

	id := globalListener.Start(StartItem{
		Name:      f.name,
		ParentID:  globalSuiteID,
		Type:      itemType,
		StartTime: start,
		Identity:  getIdentityBuilder().fixture(f.name, f.node),
	})
	if id != "" {
		msg := fmt.Sprintf(
			"%s %s\n\nLocation: %s:%d\n\nFailure:\n%s%s",
			f.name,
			status,
			where.FileName,
			where.LineNumber,
			f.message,
			notRunNote(f, notRun),
		)
		if err := globalListener.SendLog(id, gorp.LogLevelError, msg, end); err != nil {
			log.Printf(
				"WARNING: failed to send the %s failure log to ReportPortal: %v",
				f.name,
				err,
			)
		}
		globalListener.FinishTest(f.name, status, id, end)
	}

	wd, _ := os.Getwd()
	appendSpecStatus(SharedSpecStatus{
		SuiteName:       getCurrentSuiteName(),
		SuiteDirectory:  wd,
		SpecName:        f.name,
		Status:          f.state,
		Fixture:         true,
		FailureMessage:  f.message,
		FailureLocation: fmt.Sprintf("%s:%d", where.FileName, where.LineNumber),
		EndTime:         end,
	})
}

// appendSpecStatus appends a spec's result to this process's status file, which the
// wrapper reads after all workers are done.
func appendSpecStatus(status SharedSpecStatus) {
	if agentState == nil || os.Getenv("RP_SESSION_ID") == "" {
		return
	}
	file := filepath.Join(
		os.TempDir(),
		fmt.Sprintf(ProcessReportFilePattern, agentState.SessionID, os.Getpid()),
	)

	resultsMutex.Lock()
	defer resultsMutex.Unlock()

	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		log.Printf("ERROR: could not open status file: %v", err)
		return
	}
	defer func() { _ = f.Close() }()

	data, err := json.Marshal(status)
	if err != nil {
		log.Printf("ERROR: could not marshal spec status: %v", err)
		return
	}
	if _, err := f.WriteString(string(data) + "\n"); err != nil {
		log.Printf("ERROR: could not write spec status: %v", err)
	}
	_ = f.Sync()
}
