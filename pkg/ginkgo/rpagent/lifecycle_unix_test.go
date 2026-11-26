//go:build !windows

package rpagent

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

// The signal handler must not finish or forget the running spec: Ginkgo still has to
// deliver it to ReportAfterEach with its failure data. It only writes the done flag.
func TestExitHandlerWritesDoneFlagAndKeepsActiveSpecs(t *testing.T) {
	sid := fmt.Sprintf("sig-%d", time.Now().UnixNano())
	oldState := agentState
	agentState = &AgentState{SessionID: sid}
	t.Cleanup(func() { agentState = oldState })
	flag := filepath.Join(os.TempDir(), fmt.Sprintf("rp_done_%s_%d.flag", sid, os.Getpid()))
	t.Cleanup(func() { _ = os.Remove(flag) })

	activeHierarchies.Store("running spec", SpecHierarchyIDs{StepID: "spec"})
	t.Cleanup(func() { activeHierarchies.Delete("running spec") })

	exitHandlerOnce = sync.Once{} // allow the handler to be installed again (e.g. with -count=2)
	setupExitHandlers()
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil { // caught by Notify
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(flag); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the done flag was not written after SIGTERM")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, ok := activeHierarchies.Load("running spec"); !ok {
		t.Error("the handler must leave the running spec for Ginkgo's own interrupt flow")
	}
}
