package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ntfy is the minimal client we need: publish a message and subscribe to a
// stream of messages. It speaks only stdlib net/http against an ntfy server.

// ntfyAction is one action button on a published notification (ntfy "actions").
// Only the http type is used (the button fires an HTTP request when tapped).
type ntfyAction struct {
	Action  string            `json:"action"` // "http"
	Label   string            `json:"label"`
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Body    string            `json:"body,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Clear   bool              `json:"clear,omitempty"` // dismiss the notification after tap
}

// ntfyPublishReq is the JSON publish body (POST to the server root). Using the
// JSON endpoint keeps action bodies that contain JSON from being mangled by the
// header-based publish format.
type ntfyPublishReq struct {
	Topic    string       `json:"topic"`
	Message  string       `json:"message,omitempty"`
	Title    string       `json:"title,omitempty"`
	Tags     []string     `json:"tags,omitempty"`
	Priority int          `json:"priority,omitempty"`
	Actions  []ntfyAction `json:"actions,omitempty"`
}

// ntfyMessage is one item from the subscribe stream.
type ntfyMessage struct {
	ID      string   `json:"id"`
	Event   string   `json:"event"` // "open" | "message" | "keepalive"
	Topic   string   `json:"topic"`
	Message string   `json:"message"`
	Title   string   `json:"title"`
	Tags    []string `json:"tags"`
}

var ntfyHTTP = &http.Client{Timeout: 10 * time.Second}

// ntfyPublish posts one notification to the server. The caller controls the
// timeout via ctx.
func ntfyPublish(ctx context.Context, server string, req ntfyPublishReq) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(server, "/")+"/", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := ntfyHTTP.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("ntfy publish: %s", resp.Status)
	}
	return nil
}

// ntfySubscribe opens a streaming subscription to one or more topics and calls
// onMessage for each "message" event until ctx is cancelled or the stream ends.
// It returns the error that ended the stream (nil on clean ctx cancel). Callers
// that want to stay subscribed wrap this in a reconnect loop.
func ntfySubscribe(ctx context.Context, server string, topics []string, onMessage func(ntfyMessage)) error {
	url := fmt.Sprintf("%s/%s/json?since=now", strings.TrimRight(server, "/"), strings.Join(topics, ","))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	// A dedicated client with no overall timeout: the stream is long-lived; ctx
	// governs its lifetime.
	resp, err := (&http.Client{}).Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("ntfy subscribe: %s", resp.Status)
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var m ntfyMessage
		if json.Unmarshal([]byte(line), &m) != nil {
			continue
		}
		if m.Event == "message" {
			onMessage(m)
		}
	}
	return sc.Err()
}
