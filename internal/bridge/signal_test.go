package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSignalKeepsOnlyTheLatest(t *testing.T) {
	dir := t.TempDir()
	if err := writeSignal(dir, "100.5", 1); err != nil {
		t.Fatal(err)
	}
	if err := writeSignal(dir, "100.5", 2); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "signal", "100_5", "1.tga")); err == nil {
		t.Fatal("old signal survived")
	}
	data, err := os.ReadFile(filepath.Join(dir, "signal", "100_5", "2.tga"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 18+8*8*4 || data[2] != 2 || data[16] != 32 {
		t.Fatalf("bad tga: len=%d type=%d depth=%d", len(data), data[2], data[16])
	}
}
