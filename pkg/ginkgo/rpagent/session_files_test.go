package rpagent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempEntries(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestContainerCacheFilesCarryTheSessionID(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("RP_SESSION_ID", "sess-1")
	lc, err := NewLaunchCoordinator("sess-1")
	if err != nil {
		t.Fatal(err)
	}

	id, err := lc.NewContainerCacheManager().
		GetOrCreate(t.Name()+":a", func() string { return "item-1" })
	if err != nil || id != "item-1" {
		t.Fatalf("GetOrCreate = %q, %v", id, err)
	}

	names := tempEntries(t)
	if len(names) != 1 || !strings.HasPrefix(names[0], "rp_cache_sess-1_") {
		t.Fatalf(
			"the cache file must be named rp_cache_<session>_<hash> and nothing else may be left: %v",
			names,
		)
	}
}

func TestAttachmentDirectoryCarriesTheSessionID(t *testing.T) {
	old := tempAttachmentDir
	t.Cleanup(func() { tempAttachmentDir = old })
	tempAttachmentDir = ""
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("RP_SESSION_ID", "sess-1")

	InitializeAttachments()

	if !strings.HasPrefix(filepath.Base(tempAttachmentDir), "rp-agent-attachments-sess-1-") {
		t.Fatalf(
			"attachment directory = %q, want rp-agent-attachments-<session>-<random>",
			tempAttachmentDir,
		)
	}
	if _, err := os.Stat(tempAttachmentDir); err != nil {
		t.Errorf("the directory must exist: %v", err)
	}
}
