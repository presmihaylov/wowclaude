package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/presmihaylov/wowclaude/internal/slots"
)

// TestFailedRequestStaysVisible: a claude failure must survive later refreshes until a new request starts.
func TestFailedRequestStaysVisible(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("needs /bin/sh")
	}
	root := t.TempDir()
	wow := filepath.Join(root, "wow")
	sv := filepath.Join(wow, "WTF", "Account", "ACC", "SavedVariables")
	for _, d := range []string{sv, AddonDir(wow), filepath.Join(root, "projects")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fake := filepath.Join(root, "claude")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho boom >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := New(Config{
		WowDir: wow, ClaudeBin: fake, StatePath: filepath.Join(root, "state.json"),
		ProjectsRoot: filepath.Join(root, "projects"), Poll: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := slots.Install(filepath.Join(wow, "Interface", "AddOns"), 2); err != nil {
		t.Fatal(err)
	}
	outbox := "WoWClaudeDB = { outbox = { { id = 1, slot = 2, session = \"\", cwd = \"\", prompt = \"hi\" } } }\n"
	if err := os.WriteFile(filepath.Join(sv, AddonName+".lua"), []byte(outbox), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := b.drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := b.refresh(); err != nil {
		t.Fatal(err)
	}
	got := readInbox(t, b)
	if !strings.Contains(got, `error = "request 1:`) {
		t.Fatalf("error cleared by refresh:\n%s", got)
	}
	outbox = strings.Replace(outbox, "id = 1", "id = 2", 1)
	if err := os.WriteFile(filepath.Join(sv, AddonName+".lua"), []byte(outbox), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := b.drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := readInbox(t, b); !strings.Contains(got, `error = "request 2:`) {
		t.Fatalf("new request did not replace the error:\n%s", got)
	}
	// A fresh install restarts seq at 1 under a new epoch; the daemon must not skip it.
	outbox = strings.Replace(outbox, "id = 2", "id = 1", 1)
	outbox = strings.Replace(outbox, "WoWClaudeDB = {", `WoWClaudeDB = { epoch = "fresh",`, 1)
	if err := os.WriteFile(filepath.Join(sv, AddonName+".lua"), []byte(outbox), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := b.drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	got = readInbox(t, b)
	if !strings.Contains(got, `error = "request 1:`) || !strings.Contains(got, `epoch = "fresh"`) {
		t.Fatalf("seq regression was ignored:\n%s", got)
	}
	if !strings.Contains(got, "lastDoneID = 1,") {
		t.Fatalf("lastDoneID missing:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(AddonDir(wow), "signal", "fresh", "1.tga")); err != nil {
		t.Fatalf("signal file missing: %v", err)
	}
	reply, err := os.ReadFile(filepath.Join(wow, "Interface", "AddOns", "WoWClaudeIn0002", "Reply.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reply), "WoWClaudeReply = {") || !strings.Contains(string(reply), `error = "request 1:`) {
		t.Fatalf("slot payload:\n%s", reply)
	}
}

func readInbox(t *testing.T, b *Bridge) string {
	t.Helper()
	data, err := os.ReadFile(b.inboxPath())
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
