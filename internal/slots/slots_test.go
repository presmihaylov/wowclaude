package slots

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallAndWrite(t *testing.T) {
	dir := t.TempDir()
	if err := Install(dir, 3); err != nil {
		t.Fatal(err)
	}
	toc, err := os.ReadFile(filepath.Join(dir, "WoWClaudeIn0002", "WoWClaudeIn0002.toc"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(toc), "## LoadOnDemand: 1") {
		t.Fatalf("not load on demand:\n%s", toc)
	}
	if err := Write(dir, 3, "WoWClaudeReply = { id = 7 }\n"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "WoWClaudeIn0003", "Reply.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "WoWClaudeReply = { id = 7 }\n" {
		t.Fatalf("payload: %q", got)
	}
	if Name(12) != "WoWClaudeIn0012" {
		t.Fatal(Name(12))
	}
}
