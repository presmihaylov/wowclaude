package claude

import (
	"reflect"
	"strings"
	"testing"
)

func TestArgs(t *testing.T) {
	got := Args(Request{Prompt: "hi", SessionID: "s1", PermissionMode: "acceptEdits"})
	want := []string{"-p", "hi", "--output-format", "stream-json", "--verbose", "--include-partial-messages", "--permission-mode", "acceptEdits", "--resume", "s1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
	if got := Args(Request{Prompt: "hi", PermissionMode: "plan"}); strings.Contains(strings.Join(got, " "), "--resume") {
		t.Fatalf("new session must not resume: %v", got)
	}
}

func TestParse(t *testing.T) {
	stream := `{"type":"system","subtype":"init","session_id":"s-9"}
{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hel"}}}
{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"lo"}}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello"}]},"session_id":"s-9"}
{"type":"result","subtype":"success","result":"Hello","session_id":"s-9","is_error":false}
`
	var deltas []string
	res, err := Parse(strings.NewReader(stream), func(s string) { deltas = append(deltas, s) })
	if err != nil {
		t.Fatal(err)
	}
	if res.SessionID != "s-9" || res.Text != "Hello" || res.IsError {
		t.Fatalf("got %+v", res)
	}
	if strings.Join(deltas, "") != "Hello" {
		t.Fatalf("deltas %v", deltas)
	}
}

func TestParseNoResult(t *testing.T) {
	if _, err := Parse(strings.NewReader(`{"type":"system","session_id":"x"}`+"\n"), nil); err == nil {
		t.Fatal("want error on missing result")
	}
}
