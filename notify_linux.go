//go:build linux

package main

import (
	"os/exec"
	"strings"
)

func showNotification(n notification) error {
	body := strings.Join(n.Lines, "\n")
	args := []string{"-a", n.AppName}
	if n.IconPath != "" {
		args = append(args, "-i", n.IconPath)
	}
	args = append(args, n.Title, body)
	return exec.Command("notify-send", args...).Run()
}
