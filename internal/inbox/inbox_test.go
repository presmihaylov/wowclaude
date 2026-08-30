package inbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func TestRenderRoundTripsThroughLua(t *testing.T) {
	ts := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	in := Inbox{
		GeneratedAt: ts,
		DefaultCwd:  "/Users/x/proj",
		LastAckedID: 7,
		Active:      &Active{OutboxID: 8, SessionID: "s1", Prompt: "run \"it\"", Partial: "line1\nline2"},
		Sessions: []Session{{
			ID: "s1", Title: `quote " and \ backslash`, Cwd: "/x", LastModified: ts,
			Messages: []Message{{"user", "hi\ttab"}, {"assistant", "ok\x01ctl"}},
		}},
	}
	src := Render(in)

	L := lua.NewState()
	defer L.Close()
	if err := L.DoString(src); err != nil {
		t.Fatalf("lua rejects output: %v\n%s", err, src)
	}
	tbl := L.GetGlobal("WoWClaudeInbox").(*lua.LTable)
	if got := tbl.RawGetString("lastAckedID"); got != lua.LNumber(7) {
		t.Fatalf("lastAckedID = %v", got)
	}
	active := tbl.RawGetString("active").(*lua.LTable)
	if got := active.RawGetString("prompt"); got != lua.LString(`run "it"`) {
		t.Fatalf("prompt = %q", got)
	}
	if got := active.RawGetString("partial"); got != lua.LString("line1\nline2") {
		t.Fatalf("partial = %q", got)
	}
	sess := tbl.RawGetString("sessions").(*lua.LTable).RawGetInt(1).(*lua.LTable)
	if got := sess.RawGetString("title"); got != lua.LString(`quote " and \ backslash`) {
		t.Fatalf("title = %q", got)
	}
	msgs := sess.RawGetString("messages").(*lua.LTable)
	if got := msgs.RawGetInt(2).(*lua.LTable).RawGetString("text"); got != lua.LString("ok\x01ctl") {
		t.Fatalf("text = %q", got)
	}
}

func TestRenderWithoutActive(t *testing.T) {
	src := Render(Inbox{})
	if strings.Contains(src, "active") {
		t.Fatal("active must be absent when nil")
	}
}

func TestWriteIsAtomic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "Inbox.lua")
	if err := Write(p, Inbox{LastAckedID: 1}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "Inbox.lua" {
		t.Fatalf("temp file left behind: %v", entries)
	}
}

func TestTrim(t *testing.T) {
	now := time.Now()
	s := []Session{
		{ID: "old", LastModified: now.Add(-time.Hour)},
		{ID: "new", LastModified: now, Messages: []Message{{"user", "1"}, {"assistant", "2"}, {"user", "3333333"}}},
	}
	got := Trim(s, 1, 2, 3)
	if len(got) != 1 || got[0].ID != "new" {
		t.Fatalf("got %+v", got)
	}
	if len(got[0].Messages) != 2 || got[0].Messages[1].Text != "333\n[...]" {
		t.Fatalf("got %+v", got[0].Messages)
	}
}
