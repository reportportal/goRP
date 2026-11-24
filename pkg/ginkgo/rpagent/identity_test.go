package rpagent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/reportportal/goRP/v5/pkg/openapi"
)

func writeModule(t *testing.T, dir, module string) {
	t.Helper()
	if err := os.WriteFile(
		filepath.Join(dir, "go.mod"),
		[]byte("module "+module+"\n\ngo 1.25\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
}

func TestIdentityBuilderPackagePathAndCodeRef(t *testing.T) {
	root := t.TempDir()
	writeModule(t, root, "example.com/app")
	pkgDir := filepath.Join(root, "internal", "calc")
	if err := os.MkdirAll(pkgDir, 0o750); err != nil {
		t.Fatal(err)
	}

	b := newIdentityBuilder(pkgDir)
	if b.pkgPath != "example.com/app/internal/calc" {
		t.Fatalf("pkgPath = %q", b.pkgPath)
	}

	id := b.spec(
		[]string{"Calculator", "Addition"},
		"adds",
		CodeLoc{FileName: filepath.Join(pkgDir, "calc_test.go"), LineNumber: 14},
	)
	want := "example.com/app/internal/calc > Calculator > Addition > adds"
	if id.TestCaseID != want {
		t.Errorf("TestCaseID = %q, want %q", id.TestCaseID, want)
	}
	if id.CodeRef != "internal/calc/calc_test.go:14" {
		t.Errorf("CodeRef = %q", id.CodeRef)
	}
}

func TestIdentityDoesNotDependOnCheckoutLocation(t *testing.T) {
	build := func() ItemIdentity {
		root := t.TempDir() // a different absolute path every time
		writeModule(t, root, "example.com/app")
		pkgDir := filepath.Join(root, "calc")
		_ = os.MkdirAll(pkgDir, 0o750)
		b := newIdentityBuilder(pkgDir)
		return b.spec(
			[]string{"C"},
			"adds",
			CodeLoc{FileName: filepath.Join(pkgDir, "c_test.go"), LineNumber: 7},
		)
	}
	if a, b := build(), build(); a != b {
		t.Errorf("identity depends on checkout path: %+v vs %+v", a, b)
	}
}

func TestIdentityWithoutGoMod(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "suite")
	_ = os.MkdirAll(dir, 0o750)
	b := newIdentityBuilder(dir)
	if b.pkgPath != "suite" {
		t.Errorf("pkgPath = %q, want base name", b.pkgPath)
	}
	if ref := b.codeRef(
		CodeLoc{FileName: "/elsewhere/x_test.go", LineNumber: 3},
	); ref != "x_test.go:3" {
		t.Errorf("file outside the module must be reduced to its base name, got %q", ref)
	}
}

func TestContainerAndSpecIdentitiesAreDistinctAndOrdered(t *testing.T) {
	b := &identityBuilder{root: "/r", pkgPath: "m/p"}
	h := []string{"A", "B"}
	locs := []CodeLoc{
		{FileName: "/r/a_test.go", LineNumber: 5},
		{FileName: "/r/a_test.go", LineNumber: 9},
	}

	c0, c1 := b.container(h, locs, 0), b.container(h, locs, 1)
	if c0.TestCaseID != "m/p > A" || c1.TestCaseID != "m/p > A > B" {
		t.Errorf("containers = %q, %q", c0.TestCaseID, c1.TestCaseID)
	}
	if c0.CodeRef != "a_test.go:5" || c1.CodeRef != "a_test.go:9" {
		t.Errorf("container codeRefs = %q, %q", c0.CodeRef, c1.CodeRef)
	}
	if s := b.spec(h, "leaf", CodeLoc{}); s.TestCaseID != "m/p > A > B > leaf" || s.CodeRef != "" {
		t.Errorf("spec = %+v", s)
	}
	if s := b.spec(nil, "top", CodeLoc{}); s.TestCaseID != "m/p > top" {
		t.Errorf("top-level spec = %q", s.TestCaseID)
	}
}

func TestSlashInNamesCannotCollideWithNesting(t *testing.T) {
	b := &identityBuilder{root: "/r", pkgPath: "m/p"}
	flat := b.spec([]string{"a/b"}, "c", CodeLoc{})
	nested := b.spec([]string{"a", "b"}, "c", CodeLoc{})
	if flat.TestCaseID == nested.TestCaseID {
		t.Errorf(
			"Describe(%q) must differ from nested Describe/Context: %q",
			"a/b",
			flat.TestCaseID,
		)
	}
}

func TestUniqueIDIsDeterministic(t *testing.T) {
	a := ItemIdentity{TestCaseID: "m/p > A > leaf"}
	b := ItemIdentity{TestCaseID: "m/p > A > leaf"}
	c := ItemIdentity{TestCaseID: "m/p > A > other"}
	if a.uniqueID() == "" || a.uniqueID() != b.uniqueID() {
		t.Errorf("same identity must give the same uniqueId")
	}
	if a.uniqueID() == c.uniqueID() {
		t.Errorf("different identities must give different uniqueIds")
	}
	if (ItemIdentity{}).uniqueID() != "" {
		t.Errorf("empty identity must not produce a uniqueId")
	}
}

func TestStepIdentityNumbersRepeatedStepsInOrder(t *testing.T) {
	parent := ItemIdentity{TestCaseID: "m/p > A > leaf", CodeRef: "a_test.go:5"}
	first, second := stepIdentity(parent, "login", 1), stepIdentity(parent, "login", 2)
	if first.TestCaseID != "m/p > A > leaf > By: login" ||
		second.TestCaseID != "m/p > A > leaf > By: login#2" {
		t.Errorf("steps = %q, %q", first.TestCaseID, second.TestCaseID)
	}
	if first.CodeRef != parent.CodeRef {
		t.Errorf("steps share the parent's codeRef")
	}
}

func TestNoteSpecIdentityWarnsOnlyForDifferentLocations(t *testing.T) {
	seenIdentities.Clear()
	warnedDup.Clear()
	t.Cleanup(func() { seenIdentities.Clear(); warnedDup.Clear() })

	noteSpecIdentity(ItemIdentity{TestCaseID: "x", CodeRef: "a.go:1"})
	noteSpecIdentity(
		ItemIdentity{TestCaseID: "x", CodeRef: "a.go:1"},
	) // same spec re-run (flake/repeat)
	if _, warned := warnedDup.Load("x"); warned {
		t.Fatalf("the same spec at the same location must not warn")
	}
	noteSpecIdentity(ItemIdentity{TestCaseID: "x", CodeRef: "a.go:9"})
	if _, warned := warnedDup.Load("x"); !warned {
		t.Errorf("two locations with one identity must warn")
	}
}

func TestHierarchyIdentityIsStableAcrossLaunches(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir()) // isolate the shared container cache files
	t.Setenv("RP_SESSION_ID", "identity-test")

	var mu sync.Mutex
	var captured []openapi.StartTestItemRQ
	var seq int
	tl := newTestListener(t, func(w http.ResponseWriter, r *http.Request) {
		var rq openapi.StartTestItemRQ
		_ = json.NewDecoder(r.Body).Decode(&rq)
		mu.Lock()
		captured = append(captured, rq)
		seq++
		id := fmt.Sprintf("item-%d", seq)
		mu.Unlock()
		writeJSON(w, `{"id":"`+id+`"}`)
	})

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	spec := GinkgoSpecReport{
		ContainerHierarchyTexts: []string{"Calculator", "Addition"},
		ContainerHierarchyLocs: []CodeLoc{
			{FileName: filepath.Join(wd, "calc_test.go"), LineNumber: 9},
			{FileName: filepath.Join(wd, "calc_test.go"), LineNumber: 11},
		},
		LeafNodeText: "adds two numbers",
		LeafNodeLoc:  CodeLoc{FileName: filepath.Join(wd, "calc_test.go"), LineNumber: 14},
	}

	launch := func(suiteID string) []openapi.StartTestItemRQ {
		mu.Lock()
		captured = nil
		mu.Unlock()
		lc, err := NewLaunchCoordinator("identity-test")
		if err != nil {
			t.Fatal(err)
		}
		ccm := lc.NewContainerCacheManager()
		ProcessSpecWithHierarchyAndReturnIDs(nil, tl, spec, suiteID, ccm, nil)
		mu.Lock()
		defer mu.Unlock()
		return append([]openapi.StartTestItemRQ(nil), captured...)
	}

	first, second := launch("suite-of-launch-1"), launch("suite-of-launch-2")

	if len(first) != 3 || len(second) != 3 {
		t.Fatalf("want 2 containers + 1 spec per launch, got %d and %d", len(first), len(second))
	}
	for i := range first {
		a, b := first[i], second[i]
		if *a.TestCaseId != *b.TestCaseId || *a.UniqueId != *b.UniqueId ||
			*a.CodeRef != *b.CodeRef {
			t.Errorf("item %d changed between launches:\n  %s | %s | %s\n  %s | %s | %s",
				i, *a.TestCaseId, *a.UniqueId, *a.CodeRef, *b.TestCaseId, *b.UniqueId, *b.CodeRef)
		}
		if a.Uuid == b.Uuid {
			t.Errorf("item %d: its own uuid must differ per launch", i)
		}
		if strings.Contains(*a.TestCaseId, "launch") || strings.Contains(*a.TestCaseId, "item-") {
			t.Errorf("item %d: testCaseId leaks a launch-specific ID: %s", i, *a.TestCaseId)
		}
	}
	if got := *first[2].CodeRef; got != "rpagent/calc_test.go:14" {
		t.Errorf("spec codeRef = %q", got)
	}
	if got := *first[0].CodeRef; got != "rpagent/calc_test.go:9" {
		t.Errorf("container codeRef = %q", got)
	}
}
