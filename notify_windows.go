//go:build windows

package main

import (
	"strings"

	"github.com/go-toast/toast"
)

func showNotification(n notification) error {
	note := toast.Notification{
		AppID:   appID,
		Title:   n.Title,
		Message: strings.Join(n.Lines, "\n"),
		Audio:   toast.Default,
	}
	if n.IconPath != "" {
		note.Icon = n.IconPath
	}
	return note.Push()
}
