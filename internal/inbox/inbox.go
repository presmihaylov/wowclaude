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
	Epoch       string
	LastDoneID  int
	Slots       int
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
	field(&b, 1, "epoch", in.Epoch)
	field(&b, 1, "lastDoneID", in.LastDoneID)
	field(&b, 1, "slots", in.Slots)
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
		writeSession(&b, 2, s)
	}
	b.WriteString("\t},\n")
	b.WriteString("}\n")
	return b.String()
}

// Reply is one finished turn, delivered live through a LoadOnDemand slot addon.
type Reply struct {
	ID      int
	Epoch   string
	Error   string
	Session *Session
}

func RenderReply(r Reply) string {
	var b strings.Builder
	b.WriteString("WoWClaudeReply = {\n")
	field(&b, 1, "id", r.ID)
	field(&b, 1, "epoch", r.Epoch)
	field(&b, 1, "error", r.Error)
	if r.Session != nil {
		b.WriteString("\tsession =\n")
		writeSession(&b, 1, *r.Session)
	}
	b.WriteString("}\n")
	return b.String()
}

func writeSession(b *strings.Builder, depth int, s Session) {
	pad := strings.Repeat("\t", depth)
	b.WriteString(pad + "{\n")
	field(b, depth+1, "id", s.ID)
	field(b, depth+1, "title", s.Title)
	field(b, depth+1, "cwd", s.Cwd)
	field(b, depth+1, "lastModified", s.LastModified.UTC().Format(time.RFC3339))
	b.WriteString(pad + "\tmessages = {\n")
	for _, m := range s.Messages {
		fmt.Fprintf(b, "%s\t\t{ role = %s, text = %s },\n", pad, quote(m.Role), quote(m.Text))
	}
	b.WriteString(pad + "\t},\n")
	b.WriteString(pad + "},\n")
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
