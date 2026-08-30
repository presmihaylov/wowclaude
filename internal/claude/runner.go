// Package claude runs the claude CLI in print mode and parses its stream-json output.
package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type Request struct {
	Prompt         string
	SessionID      string
	Cwd            string
	PermissionMode string
}

type Result struct {
	SessionID string
	Text      string
	IsError   bool
}

// Args builds the CLI argument list; kept separate so tests can pin the exact flags.
func Args(r Request) []string {
	args := []string{
		"-p", r.Prompt,
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--permission-mode", r.PermissionMode,
	}
	if r.SessionID != "" {
		args = append(args, "--resume", r.SessionID)
	}
	return args
}

// Run blocks until the turn ends, calling onDelta with each streamed text chunk.
func Run(ctx context.Context, bin string, r Request, onDelta func(string)) (Result, error) {
	cmd := exec.CommandContext(ctx, bin, Args(r)...)
	cmd.Dir = r.Cwd
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, fmt.Errorf("stdout pipe: %w", err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("start %s: %w", bin, err)
	}
	res, perr := Parse(stdout, onDelta)
	werr := cmd.Wait()
	if perr != nil {
		return Result{}, fmt.Errorf("parse output: %w", perr)
	}
	if werr != nil {
		return Result{}, fmt.Errorf("%s exited: %w: %s", bin, werr, strings.TrimSpace(stderr.String()))
	}
	return res, nil
}

type event struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
	Result    string `json:"result"`
	IsError   bool   `json:"is_error"`
	Event     struct {
		Type  string `json:"type"`
		Delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
	} `json:"event"`
}

// Parse reads stream-json lines until the result event.
func Parse(r io.Reader, onDelta func(string)) (Result, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	var res Result
	sawResult := false
	for sc.Scan() {
		var ev event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			return Result{}, fmt.Errorf("bad line %q: %w", truncate(sc.Text()), err)
		}
		if ev.SessionID != "" {
			res.SessionID = ev.SessionID
		}
		switch ev.Type {
		case "stream_event":
			if ev.Event.Delta.Type == "text_delta" && onDelta != nil {
				onDelta(ev.Event.Delta.Text)
			}
		case "result":
			res.Text = ev.Result
			res.IsError = ev.IsError
			sawResult = true
		}
	}
	if err := sc.Err(); err != nil {
		return Result{}, fmt.Errorf("read: %w", err)
	}
	if !sawResult {
		return Result{}, fmt.Errorf("stream ended without a result event")
	}
	return res, nil
}

func truncate(s string) string {
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}
