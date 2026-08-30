// Package sessions reads Claude Code session transcripts from ~/.claude/projects.
package sessions

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Message struct {
	Role string
	Text string
}

type Session struct {
	ID           string
	Title        string
	Cwd          string
	LastModified time.Time
	Path         string
}

// Root returns the projects directory, honoring CLAUDE_CONFIG_DIR like the CLI does.
func Root() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "projects"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// Scan lists every session under root, newest first.
func Scan(root string) ([]Session, error) {
	paths, err := filepath.Glob(filepath.Join(root, "*", "*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("glob sessions: %w", err)
	}
	var out []Session
	for _, p := range paths {
		s, err := readHeader(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		if s.ID == "" {
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastModified.After(out[j].LastModified) })
	return out, nil
}

type line struct {
	Type        string          `json:"type"`
	SessionID   string          `json:"sessionId"`
	Cwd         string          `json:"cwd"`
	IsSidechain bool            `json:"isSidechain"`
	IsMeta      bool            `json:"isMeta"`
	Summary     string          `json:"summary"`
	AITitle     string          `json:"aiTitle"`
	CustomTitle string          `json:"customTitle"`
	Message     json.RawMessage `json:"message"`
}

type message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type block struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// readHeader scans the whole file because titles land at the end, but keeps only metadata.
func readHeader(path string) (Session, error) {
	st, err := os.Stat(path)
	if err != nil {
		return Session{}, fmt.Errorf("stat: %w", err)
	}
	s := Session{Path: path, LastModified: st.ModTime()}
	firstPrompt := ""
	err = eachLine(path, func(l line) {
		if s.ID == "" && l.SessionID != "" {
			s.ID = l.SessionID
		}
		if s.Cwd == "" && l.Cwd != "" {
			s.Cwd = l.Cwd
		}
		switch l.Type {
		case "custom-title":
			s.Title = l.CustomTitle
		case "ai-title":
			if s.Title == "" {
				s.Title = l.AITitle
			}
		case "summary":
			if s.Title == "" {
				s.Title = l.Summary
			}
		case "user":
			if firstPrompt != "" || l.IsSidechain || l.IsMeta {
				return
			}
			firstPrompt = firstText(l.Message)
		}
	})
	if err != nil {
		return Session{}, err
	}
	if s.Title == "" {
		s.Title = firstPrompt
	}
	if s.ID == "" {
		s.ID = strings.TrimSuffix(filepath.Base(path), ".jsonl")
	}
	return s, nil
}

// Conversation returns the user and assistant text turns of one transcript.
func Conversation(path string) ([]Message, error) {
	var out []Message
	err := eachLine(path, func(l line) {
		if l.IsSidechain || l.IsMeta {
			return
		}
		if l.Type != "user" && l.Type != "assistant" {
			return
		}
		text := allText(l.Message)
		if text == "" {
			return
		}
		out = append(out, Message{Role: l.Type, Text: text})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func eachLine(path string, fn func(line)) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var l line
		// A partial trailing line while the CLI is mid-write is the only expected bad line.
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			continue
		}
		fn(l)
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	return nil
}

func firstText(raw json.RawMessage) string {
	texts := textBlocks(raw)
	if len(texts) == 0 {
		return ""
	}
	return texts[0]
}

func allText(raw json.RawMessage) string {
	return strings.Join(textBlocks(raw), "\n")
}

// textBlocks skips tool_use, tool_result and thinking blocks; the addon only shows prose.
func textBlocks(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var m message
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	var s string
	if err := json.Unmarshal(m.Content, &s); err == nil {
		return []string{s}
	}
	var blocks []block
	if err := json.Unmarshal(m.Content, &blocks); err != nil {
		return nil
	}
	var out []string
	for _, b := range blocks {
		if b.Type != "text" || strings.TrimSpace(b.Text) == "" {
			continue
		}
		out = append(out, b.Text)
	}
	return out
}
