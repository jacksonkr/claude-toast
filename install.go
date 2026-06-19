package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// selfExe is the absolute path to the running executable.
func selfExe() string {
	p, err := os.Executable()
	if err != nil {
		return "claude-toast"
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// trayExe is the executable used for the tray. On Windows that is the
// GUI-subsystem sibling claude-toast-tray.exe (no console flash); elsewhere the
// same binary runs the tray.
func trayExe() string {
	exe := selfExe()
	if runtime.GOOS == "windows" {
		cand := filepath.Join(filepath.Dir(exe), "claude-toast-tray.exe")
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return exe
}

func runInstall() error {
	if err := writeHooks(selfExe(), true); err != nil {
		return fmt.Errorf("registering Claude Code hooks: %w", err)
	}
	if err := enableAutostart(); err != nil {
		return fmt.Errorf("enabling autostart: %w", err)
	}
	if err := registerBranding(); err != nil {
		fmt.Fprintln(os.Stderr, "warning: branding registration:", err)
	}
	if err := startTrayNow(); err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not start the tray now:", err)
	}

	fmt.Println("claude-toast installed.")
	fmt.Println("  hooks: ", filepath.Join(claudeDir(), "settings.json"))
	fmt.Println("  tray:   autostarts at login (started now)")
	fmt.Println("Run `claude-toast test` to fire a sample notification.")
	return nil
}

func runUninstall() error {
	if err := writeHooks("", false); err != nil {
		return fmt.Errorf("removing hooks: %w", err)
	}
	if err := disableAutostart(); err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
	if err := unregisterBranding(); err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
	fmt.Println("claude-toast uninstalled. Quit the running tray from its menu (or it exits at next login).")
	return nil
}

// writeHooks adds (or, when add is false, removes) the claude-toast Notification
// and Stop hooks in ~/.claude/settings.json, preserving everything else.
func writeHooks(exe string, add bool) error {
	path := filepath.Join(claudeDir(), "settings.json")

	settings := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	for _, event := range []string{"Notification", "Stop"} {
		list := removeOurHooks(toSlice(hooks[event]))
		if add {
			cmd := fmt.Sprintf("%s hook --event %s", quoteExe(exe), event)
			list = append(list, map[string]any{
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": cmd,
						"timeout": 15,
					},
				},
			})
		}
		if len(list) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = list
		}
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}

func toSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

// removeOurHooks drops any hook group whose command references claude-toast, so
// install is idempotent and uninstall is clean.
func removeOurHooks(list []any) []any {
	out := make([]any, 0, len(list))
	for _, item := range list {
		if !isOurHookGroup(item) {
			out = append(out, item)
		}
	}
	return out
}

func isOurHookGroup(item any) bool {
	m, ok := item.(map[string]any)
	if !ok {
		return false
	}
	for _, h := range toSlice(m["hooks"]) {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if cmd, _ := hm["command"].(string); strings.Contains(cmd, "claude-toast") {
			return true
		}
	}
	return false
}

func quoteExe(exe string) string {
	if strings.ContainsAny(exe, " \t") {
		return `"` + exe + `"`
	}
	return exe
}
