package outbox

import (
	"os"
	"path/filepath"
	"testing"
)

const saved = `
WoWClaudeDB = {
	["seq"] = 2,
	["outbox"] = {
		{
			["id"] = 2,
			["prompt"] = "second \"quoted\"\nline",
			["session"] = "",
			["cwd"] = "",
		}, -- [1]
		{
			["id"] = 1,
			["prompt"] = "first",
			["session"] = "abc",
			["cwd"] = "/x",
		}, -- [2]
	},
}
`

func TestParse(t *testing.T) {
	p := filepath.Join(t.TempDir(), "WoWClaude.lua")
	if err := os.WriteFile(p, []byte(saved), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Parse(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("want sorted ids, got %+v", got)
	}
	if got[0].SessionID != "abc" || got[0].Cwd != "/x" || got[0].Prompt != "first" {
		t.Fatalf("got %+v", got[0])
	}
	if got[1].Prompt != "second \"quoted\"\nline" {
		t.Fatalf("got %q", got[1].Prompt)
	}
}

func TestParseEmptyOutbox(t *testing.T) {
	p := filepath.Join(t.TempDir(), "WoWClaude.lua")
	if err := os.WriteFile(p, []byte("WoWClaudeDB = {\n\t[\"seq\"] = 0,\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Parse(p)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestParseRejectsCode(t *testing.T) {
	p := filepath.Join(t.TempDir(), "WoWClaude.lua")
	if err := os.WriteFile(p, []byte(`WoWClaudeDB = os.getenv("HOME")`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(p); err == nil {
		t.Fatal("want error: libs are not loaded")
	}
}
