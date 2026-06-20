package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAskText(t *testing.T) {
	cases := []struct{ name, event, message, want string }{
		{"stop ignores message", "Stop", "ignored", "Finished responding"},
		{"notification with message", "Notification", "Claude needs permission", "Claude needs permission"},
		{"notification empty falls back", "Notification", "", "Waiting for your input"},
		{"unknown event echoes", "SessionStart", "", "SessionStart"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := askText(c.event, c.message); got != c.want {
				t.Errorf("askText(%q,%q) = %q, want %q", c.event, c.message, got, c.want)
			}
		})
	}
}

func TestTitleFromCwd(t *testing.T) {
	if got := titleFromCwd(""); got != "Claude Code" {
		t.Errorf("empty cwd = %q, want Claude Code", got)
	}
	cwd := filepath.Join("home", "jackson", "myproj")
	if got := titleFromCwd(cwd); got != "myproj" {
		t.Errorf("titleFromCwd(%q) = %q, want myproj", cwd, got)
	}
}

func TestFirstWords(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"one two three four five six", 5, "one two three four five"},
		{"only three words", 5, "only three words"},
		{"", 5, ""},
		{"   spaced    out  ", 2, "spaced out"},
	}
	for _, c := range cases {
		if got := firstWords(c.in, c.n); got != c.want {
			t.Errorf("firstWords(%q,%d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

func TestCleanMessage(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  hello   world ", "hello world"},
		{"line1\n\nline2\tline3", "line1 line2 line3"},
		{"<command-name>/review</command-name> extra args", "/review"},
	}
	for _, c := range cases {
		if got := cleanMessage(c.in); got != c.want {
			t.Errorf("cleanMessage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestToastLines(t *testing.T) {
	cases := []struct {
		name                      string
		renameTitle, lastMsg, ask string
		want                      []string
	}{
		{"ask only", "", "", "Finished responding", []string{"Finished responding"}},
		{"message + ask", "", "fix the login bug please now later", "Finished responding",
			[]string{"fix the login bug please", "Finished responding"}},
		{"title + message + ask", "Refactor auth", "fix the login bug please now", "Waiting for your input",
			[]string{"Refactor auth", "fix the login bug please", "Waiting for your input"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := toastLines(c.renameTitle, c.lastMsg, c.ask); !reflect.DeepEqual(got, c.want) {
				t.Errorf("toastLines = %#v, want %#v", got, c.want)
			}
		})
	}
}

func TestLastUserMessage(t *testing.T) {
	if got := lastUserMessage(""); got != "" {
		t.Errorf("empty path = %q, want empty", got)
	}
	if got := lastUserMessage(filepath.Join(t.TempDir(), "nope.jsonl")); got != "" {
		t.Errorf("missing file = %q, want empty", got)
	}

	// Scanning from the end: the trailing tool_result (array content) is skipped,
	// the isMeta line is skipped, and the most recent string user message wins.
	transcript := `{"type":"assistant","message":{"content":"hello there"}}
{"type":"user","isMeta":true,"message":{"content":"meta line"}}
{"type":"user","message":{"content":"  the real question  "}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"output"}]}}
`
	p := filepath.Join(t.TempDir(), "t.jsonl")
	if err := os.WriteFile(p, []byte(transcript), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := lastUserMessage(p); got != "the real question" {
		t.Errorf("lastUserMessage = %q, want %q", got, "the real question")
	}
}
