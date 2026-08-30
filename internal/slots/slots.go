// Package slots manages the LoadOnDemand addons that carry replies into a running client.
package slots

import (
	"fmt"
	"os"
	"path/filepath"
)

const Prefix = "WoWClaudeIn"

// Name is the addon folder for one slot; the addon side formats the same string.
func Name(slot int) string {
	return fmt.Sprintf("%s%04d", Prefix, slot)
}

// Install creates every slot addon; the client only lists addons that exist when it starts.
func Install(addonsDir string, n int) error {
	for i := 1; i <= n; i++ {
		dir := filepath.Join(addonsDir, Name(i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("slot dir: %w", err)
		}
		for _, toc := range []struct{ file, iface string }{{Name(i) + ".toc", "11200"}, {Name(i) + "_Vanilla.toc", "11509"}} {
			body := fmt.Sprintf("## Interface: %s\n## Title: WoWClaude reply slot %d\n## LoadOnDemand: 1\n## Dependencies: WoWClaude\n\nReply.lua\n", toc.iface, i)
			if err := os.WriteFile(filepath.Join(dir, toc.file), []byte(body), 0o644); err != nil {
				return fmt.Errorf("slot toc: %w", err)
			}
		}
		if err := Write(addonsDir, i, "WoWClaudeReply = nil\n"); err != nil {
			return err
		}
	}
	return nil
}

// Write replaces the slot payload atomically; the client reads it fresh when LoadAddOn runs.
func Write(addonsDir string, slot int, lua string) error {
	dir := filepath.Join(addonsDir, Name(slot))
	tmp := filepath.Join(dir, ".Reply.lua.tmp")
	if err := os.WriteFile(tmp, []byte(lua), 0o644); err != nil {
		return fmt.Errorf("write slot %d: %w", slot, err)
	}
	if err := os.Rename(tmp, filepath.Join(dir, "Reply.lua")); err != nil {
		return fmt.Errorf("rename slot %d: %w", slot, err)
	}
	return nil
}
