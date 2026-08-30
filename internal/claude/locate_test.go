package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewestVersion(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []string{"2.1.9", "2.1.247", "2.1.30", "junk"} {
		if err := os.MkdirAll(filepath.Join(dir, v), 0o755); err != nil {
			t.Fatal(err)
		}
		if v != "junk" {
			if err := os.WriteFile(filepath.Join(dir, v, exeName()), nil, 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	got := newestVersion(dir)
	want := filepath.Join(dir, "2.1.247", exeName())
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if newestVersion(filepath.Join(dir, "missing")) != "" {
		t.Fatal("missing dir should yield empty")
	}
}
