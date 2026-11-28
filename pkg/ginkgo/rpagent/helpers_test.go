package rpagent

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
)

func TestPriorityLabelHelpers(t *testing.T) {
	if got := Priority(3); got != "rp.priority:3" {
		t.Errorf("Priority(3) = %q", got)
	}
	for _, bad := range []int{-1, 5, 99} {
		if got := Priority(bad); got != "rp.priority:1" {
			t.Errorf("Priority(%d) = %q, want the default level", bad, got)
		}
	}
	for level, want := range map[int]string{0: "P0", 4: "P4", 9: "P1", -2: "P1"} {
		if got := GetPriorityLabel(level); got != want {
			t.Errorf("GetPriorityLabel(%d) = %q, want %q", level, got, want)
		}
	}
}

func TestTestReferenceLabelIsSafeForGinkgo(t *testing.T) {
	if got := TestReference(""); got != "rp.ref:unset" {
		t.Errorf("an empty reference = %q, want the placeholder Ginkgo accepts", got)
	}
	if got := TestReference("PAY-TC-001"); got != "rp.ref:PAY-TC-001" {
		t.Errorf("TestReference = %q", got)
	}
	// Characters Ginkgo parses specially in a label would break it.
	if got := TestReference("a&b|c!d,e(f)g/h"); got != "rp.ref:a_b_c_d_e_f_g_h" {
		t.Errorf("TestReference with special characters = %q", got)
	}
}

func TestExtractTestReferenceID(t *testing.T) {
	spec := func(container [][]string, leaf ...string) types.SpecReport {
		return types.SpecReport{ContainerHierarchyLabels: container, LeafNodeLabels: leaf}
	}
	if got := ExtractTestReferenceIdFromReportEntry(spec(nil)); got != "" {
		t.Errorf("no label = %q, want empty", got)
	}
	if got := ExtractTestReferenceIdFromReportEntry(
		spec(nil, "smoke", TestReference("X-1")),
	); got != "X-1" {
		t.Errorf("leaf label = %q", got)
	}
	if got := ExtractTestReferenceIdFromReportEntry(
		spec([][]string{{TestReference("C-9")}}),
	); got != "C-9" {
		t.Errorf("container label = %q", got)
	}
	// Several labels: the first is used (a warning goes to stderr).
	if got := ExtractTestReferenceIdFromReportEntry(
		spec([][]string{{TestReference("FIRST")}}, TestReference("SECOND")),
	); got != "FIRST" {
		t.Errorf("two labels = %q, want the first", got)
	}
}

func TestDescribeTableHelpers(t *testing.T) {
	withTable := types.SpecReport{
		ContainerHierarchyTexts: []string{"Calculator", "string ops", "input='a'"},
		LeafNodeText:            "input='a' expected='A'",
	}
	if got := GetDescribeTableName(withTable); got != "Calculator" {
		t.Errorf("table name = %q, want the first container without '='", got)
	}
	onlyEntries := types.SpecReport{ContainerHierarchyTexts: []string{"k='v'", "k2='v2'"}}
	if got := GetDescribeTableName(onlyEntries); got != "k2='v2'" {
		t.Errorf("table name = %q, want the last container", got)
	}
	if got := GetDescribeTableName(types.SpecReport{}); got != "Unknown DescribeTable" {
		t.Errorf("table name without containers = %q", got)
	}

	var out string
	// LogParameterExtraction prints to stdout; capture it.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	LogParameterExtraction(withTable, []*Parameter{{Key: "input", Value: "a"}})
	LogParameterExtraction(withTable, nil) // prints nothing
	_ = w.Close()
	os.Stdout = old
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out = string(buf[:n])
	if !strings.Contains(out, "Extracted 1 parameter(s) from: input='a' expected='A'") ||
		!strings.Contains(out, "- input: a") {
		t.Errorf("LogParameterExtraction printed %q", out)
	}
}

func TestExtractByStatementsFromSpec(t *testing.T) {
	spec := GinkgoSpecReport{SpecEvents: []SpecEvent{
		{SpecEventType: "By", Message: "first"},
		{SpecEventType: "Node", Message: "not a step"},
		{SpecEventType: "By", Message: ""}, // a By() without text
		{SpecEventType: "By", Message: "second"},
	}}
	got := ExtractByStatementsFromSpec(spec)
	if len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Errorf("By statements = %v", got)
	}
}

// AddAttachment, Log and the labels only do something inside a running Ginkgo spec, so these
// run real specs. Ginkgo allows RunSpecs once per process: the run is shared by the tests
// below and guarded, so `go test -count=N` still works.
var (
	dslOnce    sync.Once
	dslOK      bool
	dslReports = map[string]types.SpecReport{}
)

var _ = ginkgo.Describe(
	"rpagent helpers inside a spec",
	ginkgo.Label(Priority(2), TestReference("DSL-1")),
	func() {
		ginkgo.ReportAfterEach(func(r types.SpecReport) { dslReports[r.LeafNodeText] = r })

		ginkgo.It("attaches a file and logs", func() {
			dir := ginkgo.GinkgoT().TempDir()
			path := filepath.Join(dir, "evidence.txt")
			if err := os.WriteFile(path, []byte("evidence"), 0o600); err != nil {
				ginkgo.Fail(err.Error())
			}
			AddAttachment(path, "the evidence")
			AddAttachment("", "an empty path is ignored")
			Log("WARN", "a warning")
		})

		ginkgo.It("reports every way an attachment can fail", func() {
			dir := ginkgo.GinkgoT().TempDir()
			AddAttachment(filepath.Join(dir, "missing.txt"), "missing")

			big := filepath.Join(dir, "big.txt")
			if err := os.WriteFile(big, []byte("0123456789"), 0o600); err != nil {
				ginkgo.Fail(err.Error())
			}
			old := attachmentConfig
			ginkgo.DeferCleanup(func() { attachmentConfig = old })
			attachmentConfig.MaxFileSize = 3
			AddAttachment(big, "too big")

			small := filepath.Join(dir, "small.txt")
			if err := os.WriteFile(small, []byte("ab"), 0o600); err != nil {
				ginkgo.Fail(err.Error())
			}
			attachmentConfig.TempDir = filepath.Join(dir, "no", "such", "dir")
			AddAttachment(small, "cannot copy")
		})

		ginkgo.It("carries the labels the agent reads", ginkgo.Label(Priority(0)), func() {})
	},
)

func runDSLSuite(t *testing.T) {
	t.Helper()
	if f := flag.Lookup("test.count"); f != nil && f.Value.String() != "1" {
		t.Skip("Ginkgo refuses to run a suite under go test -count>1")
	}
	dslOnce.Do(func() {
		dslOK = ginkgo.RunSpecs(t, "rpagent helpers")
	})
	if !dslOK {
		t.Fatal("the Ginkgo specs that exercise the helpers failed")
	}
}

func entryValues(r types.SpecReport) (names, values []string) {
	for _, e := range r.ReportEntries {
		names = append(names, e.Name)
		values = append(values, e.StringRepresentation())
		if e.Visibility != types.ReportEntryVisibilityNever {
			// every entry of the agent is plumbing and must stay out of Ginkgo's output
			names = append(names, "VISIBLE:"+e.Name)
		}
	}
	return names, values
}

func TestHelpersAttachAndLogInsideASpec(t *testing.T) {
	runDSLSuite(t)
	defer CleanupAttachments()
	r, ok := dslReports["attaches a file and logs"]
	if !ok {
		t.Fatal("the spec did not run")
	}
	names, values := entryValues(r)
	joined := strings.Join(values, "\n")
	if !strings.Contains(joined, attachmentPrefix) {
		t.Errorf("no attachment entry: %v", values)
	}
	if !strings.Contains(strings.Join(names, "\n"), "RP_LOG#rp_log#WARN#a warning") {
		t.Errorf("no log entry: %v", names)
	}
	for _, n := range names {
		if strings.HasPrefix(n, "VISIBLE:") {
			t.Errorf("an agent entry is visible in Ginkgo's output: %s", n)
		}
	}
	for _, e := range r.ReportEntries {
		if v := e.StringRepresentation(); strings.HasPrefix(v, attachmentPrefix) {
			tmp, orig, msg, err := ParseAttachmentPayload(v)
			if err != nil || orig != "evidence.txt" || msg != "the evidence" {
				t.Errorf("payload = %q %q %q %v", tmp, orig, msg, err)
			}
			if b, rerr := os.ReadFile(tmp); rerr != nil || string(b) != "evidence" {
				t.Errorf("the staged copy is not the file: %q, %v", b, rerr)
			}
		}
	}
}

func TestHelpersReportEveryAttachmentFailureInsideASpec(t *testing.T) {
	runDSLSuite(t)
	r, ok := dslReports["reports every way an attachment can fail"]
	if !ok {
		t.Fatal("the spec did not run")
	}
	_, values := entryValues(r)
	joined := strings.Join(values, "\n")
	for _, want := range []string{
		attachmentErrorPrefix,
		`"reason":"cannot access the file: no such file or directory"`,
		`"reason":"the file is 10 bytes, the limit is 3 bytes"`,
		`"reason":"cannot copy the file: `,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("attachment error entries lack %q:\n%s", want, joined)
		}
	}
	if got := strings.Count(joined, attachmentErrorPrefix); got != 3 {
		t.Errorf("error entries = %d, want 3", got)
	}
}

func TestLabelsInsideASpecAreReadByTheAgent(t *testing.T) {
	runDSLSuite(t)
	r, ok := dslReports["carries the labels the agent reads"]
	if !ok {
		t.Fatal("the spec did not run")
	}
	if level, set := ExtractPriorityFromReportEntry(r); level != 0 || !set {
		t.Errorf("priority = %d (set %v), want the leaf's P0", level, set)
	}
	if got := ExtractTestReferenceIdFromReportEntry(r); got != "DSL-1" {
		t.Errorf("reference = %q, want the container's", got)
	}
}
