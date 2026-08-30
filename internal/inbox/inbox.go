// Package inbox writes the Lua data file the addon reads on /reload.
package inbox

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	Messages     []Message
}

// Active describes the request the daemon is running right now, with streamed partial text.
type Active struct {
	OutboxID  int
	SessionID string
	Prompt    string
	Partial   string
}

type Inbox struct {
	GeneratedAt time.Time
	DefaultCwd  string
	LastAckedID int
	Active      *Active
	Error       string
	Sessions    []Session
}

// Write replaces path atomically so a /reload never reads a half-written file.
func Write(path string, in Inbox) error {
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	if err := os.WriteFile(tmp, []byte(Render(in)), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename to %s: %w", path, err)
	}
	return nil
}

func Render(in Inbox) string {
	var b strings.Builder
	b.WriteString("WoWClaudeInbox = {\n")
	field(&b, 1, "generatedAt", in.GeneratedAt.UTC().Format(time.RFC3339))
	field(&b, 1, "defaultCwd", in.DefaultCwd)
	field(&b, 1, "lastAckedID", in.LastAckedID)
	field(&b, 1, "error", in.Error)
	if in.Active != nil {
		b.WriteString("\tactive = {\n")
		field(&b, 2, "outboxID", in.Active.OutboxID)
		field(&b, 2, "sessionID", in.Active.SessionID)
		field(&b, 2, "prompt", in.Active.Prompt)
		field(&b, 2, "partial", in.Active.Partial)
		b.WriteString("\t},\n")
	}
	b.WriteString("\tsessions = {\n")
	for _, s := range in.Sessions {
		b.WriteString("\t\t{\n")
		field(&b, 3, "id", s.ID)
		field(&b, 3, "title", s.Title)
		field(&b, 3, "cwd", s.Cwd)
		field(&b, 3, "lastModified", s.LastModified.UTC().Format(time.RFC3339))
		b.WriteString("\t\t\tmessages = {\n")
		for _, m := range s.Messages {
			fmt.Fprintf(&b, "\t\t\t\t{ role = %s, text = %s },\n", quote(m.Role), quote(m.Text))
		}
		b.WriteString("\t\t\t},\n")
		b.WriteString("\t\t},\n")
	}
	b.WriteString("\t},\n")
	b.WriteString("}\n")
	return b.String()
}

func field(b *strings.Builder, depth int, key string, v any) {
	b.WriteString(strings.Repeat("\t", depth))
	b.WriteString(key)
	b.WriteString(" = ")
	switch x := v.(type) {
	case string:
		b.WriteString(quote(x))
	case int:
		b.WriteString(strconv.Itoa(x))
	default:
		panic(fmt.Sprintf("inbox: unsupported field type %T", v))
	}
	b.WriteString(",\n")
}

// quote emits a Lua string literal; WoW reads UTF-8 so only control and quote bytes are escaped.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\n':
			b.WriteString("\\n")
		case c == '\r':
			b.WriteString("\\r")
		case c == '\t':
			b.WriteString("\\t")
		case c < 0x20 || c == 0x7f:
			fmt.Fprintf(&b, "\\%03d", c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Trim keeps the newest sessions and the tail of each conversation so the file stays small.
func Trim(sessions []Session, maxSessions, maxMessages, maxChars int) []Session {
	sort.SliceStable(sessions, func(i, j int) bool { return sessions[i].LastModified.After(sessions[j].LastModified) })
	if len(sessions) > maxSessions {
		sessions = sessions[:maxSessions]
	}
	for i := range sessions {
		msgs := sessions[i].Messages
		if len(msgs) > maxMessages {
			msgs = msgs[len(msgs)-maxMessages:]
		}
		for j := range msgs {
			if len(msgs[j].Text) > maxChars {
				msgs[j].Text = msgs[j].Text[:maxChars] + "\n[...]"
			}
		}
		sessions[i].Messages = msgs
	}
	return sessions
}
