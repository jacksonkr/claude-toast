package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// hookPayload is the JSON Claude Code passes on stdin to a hook command.
type hookPayload struct {
	CWD            string `json:"cwd"`
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Message        string `json:"message"`
	HookEventName  string `json:"hook_event_name"`
}

// runHook is invoked by Claude Code on the Notification and Stop events.
func runHook(args []string) {
	event := "Notification"
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--event" && i+1 < len(args):
			event = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--event="):
			event = strings.TrimPrefix(args[i], "--event=")
		}
	}

	var p hookPayload
	if data, _ := io.ReadAll(os.Stdin); len(data) > 0 {
		_ = json.Unmarshal(data, &p)
	}

	if isPaused() {
		return
	}
	_ = showNotification(buildNotification(event, p))
}

// buildNotification renders the toast text (top to bottom):
//  1. the /rename session title, if one was set
//  2. the first 5 words of the latest user message
//  3. what Claude is asking / its status
//
// Lines 1 and 2 are skipped when absent; line 3 is always present. The window
// title is the project (last path segment of cwd).
func buildNotification(event string, p hookPayload) notification {
	var lines []string
	if t := renamedTitle(p.SessionID); t != "" {
		lines = append(lines, t)
	}
	if msg := lastUserMessage(p.TranscriptPath); msg != "" {
		lines = append(lines, firstWords(msg, 5))
	}

	var ask string
	switch event {
	case "Stop":
		ask = "Finished responding"
	case "Notification":
		if p.Message != "" {
			ask = p.Message
		} else {
			ask = "Waiting for your input"
		}
	default:
		ask = event
	}
	lines = append(lines, ask)

	title := "Claude Code"
	if p.CWD != "" {
		title = filepath.Base(p.CWD)
	}

	return notification{
		AppName:  "Claude Toast",
		Title:    title,
		Lines:    lines,
		IconPath: iconFilePath(),
	}
}

var (
	cmdNameRe = regexp.MustCompile(`<command-name>([^<]+)</command-name>`)
	wsRe      = regexp.MustCompile(`\s+`)
)

// lastUserMessage returns the most recent user-typed message from the live
// transcript, cleaned but not truncated. tool_result rows are also type=user
// but carry an array content, so only string content counts. Slash-command
// echoes are reduced to just the command name.
func lastUserMessage(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || !strings.Contains(line, `"type":"user"`) {
			continue
		}
		var row struct {
			Type    string `json:"type"`
			IsMeta  bool   `json:"isMeta"`
			IsSide  bool   `json:"isSidechain"`
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(line), &row) != nil {
			continue
		}
		if row.Type != "user" || row.IsMeta || row.IsSide {
			continue
		}
		var s string
		if json.Unmarshal(row.Message.Content, &s) == nil && strings.TrimSpace(s) != "" {
			return cleanMessage(s)
		}
	}
	return ""
}

func cleanMessage(s string) string {
	s = strings.TrimSpace(s)
	if m := cmdNameRe.FindStringSubmatch(s); m != nil {
		s = strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(wsRe.ReplaceAllString(s, " "))
}

func firstWords(s string, n int) string {
	f := strings.Fields(s)
	if len(f) > n {
		f = f[:n]
	}
	return strings.Join(f, " ")
}

// renamedTitle returns the title set via /rename for this session, if any.
// Claude Code stores it as the `name` field in ~/.claude/sessions/<pid>.json.
// We deliberately do not fall back to the auto-generated summary.
func renamedTitle(sid string) string {
	if sid == "" {
		return ""
	}
	dir := filepath.Join(claudeDir(), "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var o struct {
			SessionID string `json:"sessionId"`
			Name      string `json:"name"`
		}
		if json.Unmarshal(data, &o) == nil && o.SessionID == sid && o.Name != "" {
			return strings.TrimSpace(o.Name)
		}
	}
	return ""
}
