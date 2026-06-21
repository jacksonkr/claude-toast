package main

import (
	"context"
	"strings"
	"time"
)

// Broadcast: when Claude needs attention on one machine, notify the user's other
// devices. Phase 1 publishes readable ntfy fields (title/message) so the phone
// and ntfy's own apps render nice toasts; a short origin tag lets the sending
// device's own tray suppress its echo.

const originTagPrefix = "ctid-"

// originID is the on-wire identity used for echo suppression: a truncation of
// the device id, kept short so the ntfy tag stays compact.
func originID(deviceID string) string {
	if len(deviceID) > 12 {
		return deviceID[:12]
	}
	return deviceID
}

func originTag(deviceID string) string { return originTagPrefix + originID(deviceID) }

func originFromTags(tags []string) string {
	for _, t := range tags {
		if strings.HasPrefix(t, originTagPrefix) {
			return strings.TrimPrefix(t, originTagPrefix)
		}
	}
	return ""
}

// publishBroadcast sends a notification to the user's other devices. It blocks up
// to 2s and ignores errors so the short-lived hook process never stalls on a
// slow or unreachable server.
func publishBroadcast(cfg config, event string, n notification) {
	ks, ok := keysetFor(cfg)
	if !ok {
		return
	}
	title := n.Title
	if cfg.DeviceName != "" {
		title += " @" + cfg.DeviceName
	}
	req := ntfyPublishReq{
		Topic:   ks.broadcastTopic(),
		Title:   title,
		Message: strings.Join(n.Lines, "\n"),
		Tags:    []string{originTag(cfg.DeviceID)},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = ntfyPublish(ctx, cfg.NtfyServer, req)
}

// broadcastToNotification turns an inbound broadcast message into a notification
// to display, or ok=false to skip it (own echo or empty).
func broadcastToNotification(cfg config, m ntfyMessage) (notification, bool) {
	if originFromTags(m.Tags) == originID(cfg.DeviceID) {
		return notification{}, false // our own broadcast echoed back
	}
	if m.Title == "" && m.Message == "" {
		return notification{}, false
	}
	return notification{
		AppName:  "Claude Toast",
		Title:    m.Title,
		Lines:    strings.Split(m.Message, "\n"),
		IconPath: iconFilePath(),
	}, true
}
