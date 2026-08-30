// Package bridge polls the addon's SavedVariables, runs claude, and rewrites Inbox.lua.
package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/presmihaylov/wowclaude/internal/claude"
	"github.com/presmihaylov/wowclaude/internal/inbox"
	"github.com/presmihaylov/wowclaude/internal/outbox"
	"github.com/presmihaylov/wowclaude/internal/sessions"
)

const (
	AddonName   = "WoWClaude"
	maxSessions = 30
	maxMessages = 40
	maxChars    = 6000
)

type Config struct {
	WowDir         string
	Account        string
	ClaudeBin      string
	DefaultCwd     string
	PermissionMode string
	StatePath      string
	ProjectsRoot   string
	Poll           time.Duration
}

type state struct {
	LastAckedID int `json:"lastAckedID"`
}

type Bridge struct {
	cfg   Config
	st    state
	mu    sync.Mutex
	inbox inbox.Inbox
	// lastErr stays in the inbox until the next request starts, so a failed turn is visible after Refresh.
	lastErr string
}

func New(cfg Config) (*Bridge, error) {
	if cfg.Account == "" {
		acct, err := detectAccount(cfg.WowDir)
		if err != nil {
			return nil, err
		}
		cfg.Account = acct
	}
	b := &Bridge{cfg: cfg}
	if err := b.loadState(); err != nil {
		return nil, err
	}
	return b, nil
}

func AddonDir(wowDir string) string {
	return filepath.Join(wowDir, "Interface", "AddOns", AddonName)
}

func (b *Bridge) inboxPath() string {
	return filepath.Join(AddonDir(b.cfg.WowDir), "Inbox.lua")
}

func (b *Bridge) savedVarsPath() string {
	return filepath.Join(b.cfg.WowDir, "WTF", "Account", b.cfg.Account, "SavedVariables", AddonName+".lua")
}

// detectAccount picks the single account folder under WTF/Account; ask for a flag when there are several.
func detectAccount(wowDir string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(wowDir, "WTF", "Account"))
	if err != nil {
		return "", fmt.Errorf("list accounts: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != "SavedVariables" {
			names = append(names, e.Name())
		}
	}
	if len(names) != 1 {
		return "", fmt.Errorf("found %d account folders %v, pass --account", len(names), names)
	}
	return names[0], nil
}

func (b *Bridge) loadState() error {
	data, err := os.ReadFile(b.cfg.StatePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}
	if err := json.Unmarshal(data, &b.st); err != nil {
		return fmt.Errorf("parse state: %w", err)
	}
	return nil
}

func (b *Bridge) saveState() error {
	data, err := json.Marshal(b.st)
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(b.cfg.StatePath), 0o755); err != nil {
		return fmt.Errorf("state dir: %w", err)
	}
	if err := os.WriteFile(b.cfg.StatePath, data, 0o644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

// Run blocks, polling SavedVariables by mtime; the game rewrites the file only on /reload or logout.
func (b *Bridge) Run(ctx context.Context) error {
	log.Printf("watching %s", b.savedVarsPath())
	log.Printf("writing  %s", b.inboxPath())
	if err := b.refresh(); err != nil {
		return err
	}
	var lastMod time.Time
	lastRefresh := time.Now()
	tick := time.NewTicker(b.cfg.Poll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
		st, err := os.Stat(b.savedVarsPath())
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("stat saved variables: %w", err)
		}
		if st.ModTime().After(lastMod) {
			lastMod = st.ModTime()
			if err := b.drain(ctx); err != nil {
				return err
			}
			lastRefresh = time.Now()
			continue
		}
		if time.Since(lastRefresh) > time.Minute {
			lastRefresh = time.Now()
			if err := b.refresh(); err != nil {
				return err
			}
		}
	}
}

func (b *Bridge) drain(ctx context.Context) error {
	reqs, err := outbox.Parse(b.savedVarsPath())
	if err != nil {
		log.Printf("outbox: %v", err)
		b.lastErr = err.Error()
		return b.refresh()
	}
	for _, r := range reqs {
		if r.ID <= b.st.LastAckedID {
			continue
		}
		if err := b.handle(ctx, r); err != nil {
			return err
		}
	}
	return b.refresh()
}

func (b *Bridge) handle(ctx context.Context, r outbox.Request) error {
	b.st.LastAckedID = r.ID
	b.lastErr = ""
	if err := b.saveState(); err != nil {
		return err
	}
	cwd := r.Cwd
	if cwd == "" {
		cwd = b.cfg.DefaultCwd
	}
	log.Printf("request %d: session=%q cwd=%s prompt=%q", r.ID, r.SessionID, cwd, truncate(r.Prompt))
	active := &inbox.Active{OutboxID: r.ID, SessionID: r.SessionID, Prompt: r.Prompt}
	if err := b.writeInbox(active); err != nil {
		return err
	}
	var partial strings.Builder
	lastWrite := time.Now()
	var writeErr error
	onDelta := func(s string) {
		partial.WriteString(s)
		if time.Since(lastWrite) < time.Second || writeErr != nil {
			return
		}
		lastWrite = time.Now()
		active.Partial = partial.String()
		writeErr = b.writeInbox(active)
	}
	res, err := claude.Run(ctx, b.cfg.ClaudeBin, claude.Request{
		Prompt: r.Prompt, SessionID: r.SessionID, Cwd: cwd, PermissionMode: b.cfg.PermissionMode,
	}, onDelta)
	if writeErr != nil {
		return writeErr
	}
	if err != nil {
		log.Printf("request %d failed: %v", r.ID, err)
		b.lastErr = fmt.Sprintf("request %d: %v", r.ID, err)
		return b.refresh()
	}
	log.Printf("request %d done: session=%s chars=%d", r.ID, res.SessionID, len(res.Text))
	if res.IsError {
		b.lastErr = fmt.Sprintf("request %d: claude reported an error: %s", r.ID, truncate(res.Text))
		return b.refresh()
	}
	return nil
}

// refresh rescans transcripts and rewrites the inbox with no active request.
func (b *Bridge) refresh() error {
	sess, err := b.loadSessions()
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.inbox.Sessions = sess
	b.mu.Unlock()
	return b.writeInbox(nil)
}

func (b *Bridge) writeInbox(active *inbox.Active) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.inbox.GeneratedAt = time.Now()
	b.inbox.DefaultCwd = b.cfg.DefaultCwd
	b.inbox.LastAckedID = b.st.LastAckedID
	b.inbox.Active = active
	b.inbox.Error = b.lastErr
	if err := inbox.Write(b.inboxPath(), b.inbox); err != nil {
		return err
	}
	return nil
}

func (b *Bridge) loadSessions() ([]inbox.Session, error) {
	list, err := sessions.Scan(b.cfg.ProjectsRoot)
	if err != nil {
		return nil, err
	}
	if len(list) > maxSessions {
		list = list[:maxSessions]
	}
	var out []inbox.Session
	for _, s := range list {
		msgs, err := sessions.Conversation(s.Path)
		if err != nil {
			return nil, err
		}
		is := inbox.Session{ID: s.ID, Title: s.Title, Cwd: s.Cwd, LastModified: s.LastModified}
		for _, m := range msgs {
			is.Messages = append(is.Messages, inbox.Message{Role: m.Role, Text: m.Text})
		}
		out = append(out, is)
	}
	return inbox.Trim(out, maxSessions, maxMessages, maxChars), nil
}

func truncate(s string) string {
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}
