package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/presmihaylov/wowclaude"
	"github.com/presmihaylov/wowclaude/internal/bridge"
	"github.com/presmihaylov/wowclaude/internal/sessions"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "wowclaude:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: wowclaude <install|serve|sessions> [flags]")
	}
	switch args[0] {
	case "install":
		return install(args[1:])
	case "serve":
		return serve(args[1:])
	case "sessions":
		return listSessions()
	}
	return fmt.Errorf("unknown command %q", args[0])
}

func install(args []string) error {
	fs_ := flag.NewFlagSet("install", flag.ContinueOnError)
	wowDir := fs_.String("wow-dir", "", "game folder, e.g. \"/Applications/World of Warcraft/_classic_era_\"")
	if err := fs_.Parse(args); err != nil {
		return err
	}
	if *wowDir == "" {
		return fmt.Errorf("--wow-dir is required")
	}
	dst := bridge.AddonDir(*wowDir)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	src, err := fs.Sub(wowclaude.Addon, "addon/WoWClaude")
	if err != nil {
		return fmt.Errorf("embedded addon: %w", err)
	}
	entries, err := fs.ReadDir(src, ".")
	if err != nil {
		return fmt.Errorf("read embedded addon: %w", err)
	}
	for _, e := range entries {
		data, err := fs.ReadFile(src, e.Name())
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", e.Name(), err)
		}
		fmt.Println("installed", filepath.Join(dst, e.Name()))
	}
	return nil
}

func serve(args []string) error {
	fs_ := flag.NewFlagSet("serve", flag.ContinueOnError)
	cfg := bridge.Config{Poll: time.Second}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}
	fs_.StringVar(&cfg.WowDir, "wow-dir", "", "game folder, e.g. \"/Applications/World of Warcraft/_classic_era_\"")
	fs_.StringVar(&cfg.Account, "account", "", "WTF/Account/<name>; auto-detected when there is one")
	fs_.StringVar(&cfg.ClaudeBin, "claude", "claude", "claude binary")
	fs_.StringVar(&cfg.DefaultCwd, "cwd", home, "working directory for new sessions")
	fs_.StringVar(&cfg.PermissionMode, "permission-mode", "acceptEdits", "claude --permission-mode; print mode cannot answer prompts")
	fs_.StringVar(&cfg.StatePath, "state", filepath.Join(home, ".wowclaude", "state.json"), "daemon state file")
	if err := fs_.Parse(args); err != nil {
		return err
	}
	if cfg.WowDir == "" {
		return fmt.Errorf("--wow-dir is required")
	}
	root, err := sessions.Root()
	if err != nil {
		return err
	}
	cfg.ProjectsRoot = root
	b, err := bridge.New(cfg)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return b.Run(ctx)
}

func listSessions() error {
	root, err := sessions.Root()
	if err != nil {
		return err
	}
	list, err := sessions.Scan(root)
	if err != nil {
		return err
	}
	for _, s := range list {
		fmt.Printf("%s  %s  %-40.40s  %s\n", s.LastModified.Format("2006-01-02 15:04"), s.ID, s.Title, s.Cwd)
	}
	return nil
}
