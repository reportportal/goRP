package rpagent

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// identitySeparator joins the parts of an item identity. " > " is used instead of "/"
// because spec texts commonly contain slashes (e.g. "GET /orders").
const identitySeparator = " > "

// uniqueIDNamespace is the fixed UUIDv5 namespace used to derive uniqueId values.
var uniqueIDNamespace = uuid.MustParse("5b0c5f0e-8f1a-4c6e-9d57-2a1f3e6b7c10")

// CodeLoc is the source location of a Ginkgo node.
type CodeLoc struct {
	FileName   string `json:"FileName"`
	LineNumber int    `json:"LineNumber"`
}

// ItemIdentity is the run-independent identity of a test item. It is built only from
// data that does not change between launches, so ReportPortal can link the same test
// across runs (history, retries, auto-analysis).
type ItemIdentity struct {
	TestCaseID string
	CodeRef    string
}

// uniqueID returns a deterministic UUID derived from the test case ID, or "" if unset.
func (i ItemIdentity) uniqueID() string {
	if i.TestCaseID == "" {
		return ""
	}
	return uuid.NewSHA1(uniqueIDNamespace, []byte(i.TestCaseID)).String()
}

// identityBuilder derives item identities for the suite running in the current directory.
type identityBuilder struct {
	root    string // directory containing go.mod
	pkgPath string // module path + package directory relative to the module root
}

// newIdentityBuilder builds an identityBuilder for the package in dir.
func newIdentityBuilder(dir string) *identityBuilder {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	root, module := findModule(dir)
	pkg := module
	if rel, err := filepath.Rel(root, dir); err == nil && rel != "." {
		pkg = module + "/" + filepath.ToSlash(rel)
	}
	return &identityBuilder{root: root, pkgPath: pkg}
}

// findModule walks up from dir looking for go.mod and returns its directory and module
// path. Without a go.mod it falls back to dir itself and the directory's base name.
func findModule(dir string) (root, module string) {
	for cur := dir; ; {
		if m := readModulePath(filepath.Join(cur, "go.mod")); m != "" {
			return cur, m
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return dir, filepath.Base(dir)
}

func readModulePath(goMod string) string {
	f, err := os.Open(goMod)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if rest, ok := strings.CutPrefix(line, "module"); ok && strings.HasPrefix(rest, " ") {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
}

// codeRef formats a location as "<path relative to the module root>:<line>".
// Files outside the module are reduced to their base name, so the value never
// depends on where the repository is checked out.
func (b *identityBuilder) codeRef(loc CodeLoc) string {
	if loc.FileName == "" {
		return ""
	}
	file := loc.FileName
	// Resolve symlinks on the directory, not the file: the module root was resolved the
	// same way, and the directory resolves even when the file itself cannot be statted.
	if dir, base := filepath.Split(file); dir != "" {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			file = filepath.Join(resolved, base)
		}
	}
	rel, err := filepath.Rel(b.root, file)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = filepath.Base(file)
	}
	return fmt.Sprintf("%s:%d", filepath.ToSlash(rel), loc.LineNumber)
}

// container returns the identity of the i-th container (Describe/Context) of a spec.
func (b *identityBuilder) container(hierarchy []string, locs []CodeLoc, i int) ItemIdentity {
	parts := append([]string{b.pkgPath}, hierarchy[:i+1]...)
	id := ItemIdentity{TestCaseID: strings.Join(parts, identitySeparator)}
	if i < len(locs) {
		id.CodeRef = b.codeRef(locs[i])
	}
	return id
}

// spec returns the identity of a spec (the It / Entry leaf).
func (b *identityBuilder) spec(hierarchy []string, leaf string, loc CodeLoc) ItemIdentity {
	parts := append([]string{b.pkgPath}, hierarchy...)
	parts = append(parts, leaf)
	return ItemIdentity{
		TestCaseID: strings.Join(parts, identitySeparator),
		CodeRef:    b.codeRef(loc),
	}
}

// fixture returns the identity of a suite-level node such as [BeforeSuite]: the package
// plus the node's own name, with the node's location as its codeRef.
func (b *identityBuilder) fixture(name string, loc CodeLoc) ItemIdentity {
	return ItemIdentity{
		TestCaseID: b.pkgPath + identitySeparator + name,
		CodeRef:    b.codeRef(loc),
	}
}

// stepIdentity returns the identity of a By() step. occurrence is 1 for the first step with a
// given text inside one spec; later steps with the same text get "#<n>". Steps are
// created in order within one spec, so this numbering is deterministic.
func stepIdentity(parent ItemIdentity, name string, occurrence int) ItemIdentity {
	id := parent.TestCaseID + identitySeparator + "By: " + name
	if occurrence > 1 {
		id = fmt.Sprintf("%s#%d", id, occurrence)
	}
	return ItemIdentity{TestCaseID: id, CodeRef: parent.CodeRef}
}

var (
	identityOnce sync.Once
	identityVal  *identityBuilder
)

// getIdentityBuilder returns the builder for the current working directory (the suite directory).
func getIdentityBuilder() *identityBuilder {
	identityOnce.Do(func() {
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}
		identityVal = newIdentityBuilder(wd)
	})
	return identityVal
}

var (
	seenIdentities sync.Map // testCaseID -> codeRef
	warnedDup      sync.Map // testCaseID -> struct{}
)

// noteSpecIdentity warns, once per identity, when two specs declared at different
// locations resolve to the same identity (same package, containers and text).
// Their ReportPortal history is then shared; renaming one separates it.
func noteSpecIdentity(id ItemIdentity) {
	prev, loaded := seenIdentities.LoadOrStore(id.TestCaseID, id.CodeRef)
	if !loaded || prev == id.CodeRef {
		return
	}
	if _, warned := warnedDup.LoadOrStore(id.TestCaseID, struct{}{}); warned {
		return
	}
	log.Printf(
		"WARNING: two specs share the name %q (%v and %s); they will share one ReportPortal "+
			"history. Rename one to keep their history separate.",
		id.TestCaseID, prev, id.CodeRef,
	)
}
