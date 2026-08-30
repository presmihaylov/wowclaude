package claude

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Locate resolves the claude binary: an explicit path or PATH entry wins; otherwise the newest desktop-app copy.
func Locate(bin string) (string, error) {
	if p, err := exec.LookPath(bin); err == nil {
		return p, nil
	}
	if bin != "claude" {
		return "", fmt.Errorf("claude binary %q not found", bin)
	}
	for _, dir := range desktopDirs() {
		if p := newestVersion(dir); p != "" {
			return p, nil
		}
	}
	return "", fmt.Errorf("claude is not on PATH and no desktop-app copy was found; pass --claude")
}

// The desktop app installs claude under a per-version folder that renames itself on every update.
func desktopDirs() []string {
	var dirs []string
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		dirs = append(dirs, filepath.Join(appdata, "Claude", "claude-code"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "Library", "Application Support", "Claude", "claude-code"))
	}
	return dirs
}

func newestVersion(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() && exists(filepath.Join(dir, e.Name(), exeName())) {
			versions = append(versions, e.Name())
		}
	}
	if len(versions) == 0 {
		return ""
	}
	sort.Slice(versions, func(i, j int) bool { return versionLess(versions[j], versions[i]) })
	return filepath.Join(dir, versions[0], exeName())
}

func exeName() string {
	if os.Getenv("APPDATA") != "" || strings.HasSuffix(os.Getenv("OS"), "Windows_NT") {
		return "claude.exe"
	}
	return "claude"
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// versionLess compares dotted numeric versions, so 2.1.10 sorts after 2.1.9.
func versionLess(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		ai, _ := strconv.Atoi(as[i])
		bi, _ := strconv.Atoi(bs[i])
		if ai != bi {
			return ai < bi
		}
	}
	return len(as) < len(bs)
}
