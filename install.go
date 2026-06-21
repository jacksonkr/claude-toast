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
	cfg := ensureInitialized()
	if err := writeHooks(cfg, true); err != nil {
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
	fmt.Println("  UID:   ", cfg.Secret)
	fmt.Println("Run `claude-toast test` to fire a sample notification.")
	fmt.Println("Link another device with `claude-toast link " + cfg.Secret + "`.")
	return nil
}

func runUninstall() error {
	if err := writeHooks(config{}, false); err != nil {
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

// hookExe is the executable hooks should invoke: the console binary. On Windows
// that is the sibling claude-toast.exe even when the caller is the GUI tray
// (claude-toast-tray.exe), so a tray-initiated rewrite still points hooks at the
// console build.
func hookExe() string {
	exe := selfExe()
	if runtime.GOOS == "windows" {
		cand := filepath.Join(filepath.Dir(exe), "claude-toast.exe")
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return exe
}

// reconcileHooks rewrites the hook entries to match the current config (used
// when the tray toggles remote-approve on/off).
func reconcileHooks(cfg config) error { return writeHooks(cfg, true) }

// writeHooks adds (or, when add is false, removes) the claude-toast hooks in
// ~/.claude/settings.json, preserving everything else. Which events are wired is
// driven by cfg (PreToolUse only when remote-approve is enabled).
func writeHooks(cfg config, add bool) error {
	path := filepath.Join(claudeDir(), "settings.json")

	settings := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
	}

	settings = applyHooks(settings, hookExe(), cfg, add)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}

// applyHooks is the pure merge: given the parsed settings map, add or remove our
// hook groups, preserving every other key and any hooks the user added. We
// always sweep all three events so a removed/disabled one is cleared; on add we
// wire Notification/Stop always and PreToolUse only when cfg.RemoteApprove. The
// PreToolUse group carries a matcher (regex over tool names) so only allowlisted
// tools invoke our blocking decision hook.
func applyHooks(settings map[string]any, exe string, cfg config, add bool) map[string]any {
	if settings == nil {
		settings = map[string]any{}
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	wanted := map[string]bool{"Notification": add, "Stop": add, "PreToolUse": add && cfg.RemoteApprove}

	for _, event := range []string{"Notification", "Stop", "PreToolUse"} {
		list := removeOurHooks(toSlice(hooks[event]))
		if wanted[event] {
			cmd := fmt.Sprintf("%s hook --event %s", quoteExe(exe), event)
			timeout := 15
			group := map[string]any{
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": cmd,
						"timeout": timeout,
					},
				},
			}
			if event == "PreToolUse" {
				group["matcher"] = strings.Join(cfg.Allowlist, "|")
				group["hooks"].([]any)[0].(map[string]any)["timeout"] = 20
			}
			list = append(list, group)
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
	return settings
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

// quoteExe renders the executable path for a hook command. Claude Code runs hook
// commands through POSIX sh (Git Bash) on Windows, where backslashes are escape
// characters and would mangle a native path (C:\...\claude-toast.exe becomes
// C:...claude-toast.exe -> command not found). Forward slashes work in both sh
// and Windows, so normalize them on Windows. Always quote so spaces are safe.
func quoteExe(exe string) string {
	if runtime.GOOS == "windows" {
		exe = strings.ReplaceAll(exe, `\`, "/")
		return `"` + exe + `"`
	}
	if strings.ContainsAny(exe, " \t") {
		return `"` + exe + `"`
	}
	return exe
}
