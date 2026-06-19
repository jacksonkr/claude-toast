package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// claudeDir is Claude Code's config directory (~/.claude).
func claudeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// configDir is where claude-toast keeps its own runtime data (extracted icon,
// pause flag): %AppData%\claude-toast, ~/Library/Application Support/claude-toast,
// or ~/.config/claude-toast.
func configDir() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		home, _ := os.UserHomeDir()
		base = home
	}
	return filepath.Join(base, "claude-toast")
}

func ensureConfigDir() (string, error) {
	d := configDir()
	return d, os.MkdirAll(d, 0o755)
}

func pauseFlagPath() string { return filepath.Join(configDir(), "paused") }

func isPaused() bool {
	_, err := os.Stat(pauseFlagPath())
	return err == nil
}

func setPaused(p bool) error {
	if p {
		if _, err := ensureConfigDir(); err != nil {
			return err
		}
		return os.WriteFile(pauseFlagPath(), []byte("paused\n"), 0o644)
	}
	if err := os.Remove(pauseFlagPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// iconFilePath extracts the embedded notification image to the config dir once
// and returns its absolute path (notify-send / go-toast want a file path).
func iconFilePath() string {
	d, err := ensureConfigDir()
	if err != nil {
		return ""
	}
	p := filepath.Join(d, "toast.png")
	if _, err := os.Stat(p); err != nil {
		if err := os.WriteFile(p, notifyIconData, 0o644); err != nil {
			return ""
		}
	}
	return p
}

// openPath opens a file or folder in the OS file manager.
func openPath(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("cmd", "/c", "start", "", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
