// Package outbox parses the addon's SavedVariables file, which the game writes on /reload.
package outbox

import (
	"fmt"
	"os"
	"sort"

	lua "github.com/yuin/gopher-lua"
)

type Request struct {
	ID        int
	SessionID string
	Cwd       string
	Prompt    string
}

// Parse evaluates the SavedVariables file in a sandboxed Lua state and returns requests by ID.
func Parse(path string) ([]Request, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()
	if err := L.DoString(string(src)); err != nil {
		return nil, fmt.Errorf("eval %s: %w", path, err)
	}
	db, ok := L.GetGlobal("WoWClaudeDB").(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("%s: WoWClaudeDB is not a table", path)
	}
	outbox, ok := db.RawGetString("outbox").(*lua.LTable)
	if !ok {
		return nil, nil
	}
	var out []Request
	var perr error
	outbox.ForEach(func(_, v lua.LValue) {
		entry, ok := v.(*lua.LTable)
		if !ok {
			perr = fmt.Errorf("%s: outbox entry is not a table", path)
			return
		}
		id, ok := entry.RawGetString("id").(lua.LNumber)
		if !ok {
			perr = fmt.Errorf("%s: outbox entry without numeric id", path)
			return
		}
		out = append(out, Request{
			ID:        int(id),
			SessionID: str(entry, "session"),
			Cwd:       str(entry, "cwd"),
			Prompt:    str(entry, "prompt"),
		})
	})
	if perr != nil {
		return nil, perr
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func str(t *lua.LTable, key string) string {
	if s, ok := t.RawGetString(key).(lua.LString); ok {
		return string(s)
	}
	return ""
}
