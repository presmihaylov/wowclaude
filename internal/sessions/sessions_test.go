package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

const transcript = `{"type":"permission-mode","permissionMode":"default","sessionId":"abc-1"}
{"type":"user","isSidechain":false,"message":{"role":"user","content":"hello there"},"cwd":"/tmp/proj","sessionId":"abc-1"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"hmm"},{"type":"text","text":"hi back"},{"type":"tool_use","name":"Bash"}]},"cwd":"/tmp/proj","sessionId":"abc-1"}
{"type":"user","isMeta":true,"message":{"role":"user","content":"meta noise"},"sessionId":"abc-1"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"x"}]},"sessionId":"abc-1"}
{"type":"user","isSidechain":true,"message":{"role":"user","content":"subagent prompt"},"sessionId":"abc-1"}
{"type":"ai-title","aiTitle":"Greeting","sessionId":"abc-1"}
not json at all
`

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestScanAndConversation(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "-tmp-proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	p := write(t, proj, "abc-1.jsonl", transcript)

	got, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 session, got %d", len(got))
	}
	s := got[0]
	if s.ID != "abc-1" || s.Cwd != "/tmp/proj" || s.Title != "Greeting" || s.Path != p {
		t.Fatalf("unexpected session %+v", s)
	}

	msgs, err := Conversation(p)
	if err != nil {
		t.Fatal(err)
	}
	want := []Message{{"user", "hello there"}, {"assistant", "hi back"}}
	if len(msgs) != len(want) {
		t.Fatalf("want %d messages, got %+v", len(want), msgs)
	}
	for i := range want {
		if msgs[i] != want[i] {
			t.Fatalf("message %d: want %+v got %+v", i, want[i], msgs[i])
		}
	}
}

func TestTitleFallsBackToFirstPrompt(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "-x")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, proj, "s.jsonl", `{"type":"user","message":{"role":"user","content":"first words"},"cwd":"/x","sessionId":"s"}`+"\n")
	got, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Title != "first words" {
		t.Fatalf("got title %q", got[0].Title)
	}
}
