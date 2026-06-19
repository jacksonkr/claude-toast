//go:build darwin

package main

import (
	"os/exec"
	"strings"
)

func showNotification(n notification) error {
	body := strings.Join(n.Lines, " — ")
	script := "display notification " + osaQuote(body) +
		" with title " + osaQuote(n.Title)
	return exec.Command("osascript", "-e", script).Run()
}

// osaQuote wraps a string as an AppleScript string literal.
func osaQuote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return "\"" + s + "\""
}
