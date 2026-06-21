package main

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

func mustParse(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return m
}

func groups(settings map[string]any, event string) []any {
	hooks, _ := settings["hooks"].(map[string]any)
	list, _ := hooks[event].([]any)
	return list
}

func TestApplyHooksAddToEmpty(t *testing.T) {
	out := applyHooks(map[string]any{}, "claude-toast", true)
	for _, ev := range []string{"Notification", "Stop"} {
		if g := groups(out, ev); len(g) != 1 {
			t.Fatalf("%s: got %d groups, want 1", ev, len(g))
		}
	}
	blob := string(mustMarshal(t, out))
	if !strings.Contains(blob, "claude-toast") || !strings.Contains(blob, "hook --event Stop") {
		t.Error("Stop hook command not written")
	}
}

func TestApplyHooksPreservesOtherKeys(t *testing.T) {
	in := mustParse(t, `{"cleanupPeriodDays":3650,"enabledPlugins":{"x":true}}`)
	out := applyHooks(in, "claude-toast", true)
	if out["cleanupPeriodDays"] == nil {
		t.Error("cleanupPeriodDays dropped")
	}
	if _, ok := out["enabledPlugins"].(map[string]any); !ok {
		t.Error("enabledPlugins dropped")
	}
}

func TestApplyHooksIdempotent(t *testing.T) {
	out := applyHooks(map[string]any{}, "claude-toast", true)
	out = applyHooks(out, "claude-toast", true) // install twice
	if g := groups(out, "Stop"); len(g) != 1 {
		t.Fatalf("after double install: %d Stop groups, want 1", len(g))
	}
}

func TestApplyHooksPreservesUserHook(t *testing.T) {
	in := mustParse(t, `{"hooks":{"Notification":[{"hooks":[{"type":"command","command":"echo hi"}]}]}}`)
	out := applyHooks(in, "claude-toast", true)
	if g := groups(out, "Notification"); len(g) != 2 {
		t.Fatalf("got %d Notification groups, want 2 (user + ours)", len(g))
	}
	blob := string(mustMarshal(t, out))
	if !strings.Contains(blob, "echo hi") {
		t.Error("user hook 'echo hi' was removed")
	}
	if !strings.Contains(blob, "hook --event Notification") {
		t.Error("our hook not added")
	}
}

func TestApplyHooksRemove(t *testing.T) {
	in := mustParse(t, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"my-script"}]}]}}`)
	in = applyHooks(in, "claude-toast", true)    // Stop: [user, ours]; Notification: [ours]
	out := applyHooks(in, "claude-toast", false) // remove ours

	if g := groups(out, "Stop"); len(g) != 1 {
		t.Fatalf("Stop after remove: %d groups, want 1 (user)", len(g))
	}
	if g := groups(out, "Notification"); len(g) != 0 {
		t.Fatalf("Notification after remove: %d groups, want 0", len(g))
	}
	blob := string(mustMarshal(t, out))
	if strings.Contains(blob, "claude-toast") {
		t.Error("our hooks not fully removed")
	}
	if !strings.Contains(blob, "my-script") {
		t.Error("user hook lost during remove")
	}
}

func TestApplyHooksRemoveEmptiesHooksKey(t *testing.T) {
	out := applyHooks(map[string]any{}, "claude-toast", true)
	out = applyHooks(out, "claude-toast", false)
	if _, ok := out["hooks"]; ok {
		t.Error("hooks key should be deleted when no hooks remain")
	}
}

func TestQuoteExe(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Claude Code runs hooks through POSIX sh, which eats backslashes; the
		// path must be forward-slashed and quoted so it survives. A backslash in
		// the result is the exact bug that made hooks silently fail.
		got := quoteExe(`C:\tools\claude-toast.exe`)
		if got != `"C:/tools/claude-toast.exe"` {
			t.Errorf("windows path = %q, want forward-slashed and quoted", got)
		}
		if strings.Contains(got, `\`) {
			t.Errorf("windows hook command must not contain a backslash, got %q", got)
		}
		return
	}
	if got := quoteExe(`/usr/local/bin/claude-toast`); got != `/usr/local/bin/claude-toast` {
		t.Errorf("no-space path should be unquoted, got %q", got)
	}
	if got := quoteExe(`/opt/My Apps/ct`); got != `"/opt/My Apps/ct"` {
		t.Errorf("spaced path = %q, want quoted", got)
	}
}

func TestApplyHooksNilSettings(t *testing.T) {
	out := applyHooks(nil, "claude-toast", true)
	if g := groups(out, "Stop"); len(g) != 1 {
		t.Fatalf("nil settings: %d Stop groups, want 1", len(g))
	}
}

func TestRemoveOurHooksIgnoresMalformed(t *testing.T) {
	list := []any{
		"not a map",                        // item not a map
		map[string]any{"hooks": []any{42}}, // inner hook not a map
		map[string]any{"hooks": []any{map[string]any{"command": "claude-toast hook --event Stop"}}}, // ours
		map[string]any{"hooks": []any{map[string]any{"command": "keepme"}}},                         // user
	}
	out := removeOurHooks(list)
	if len(out) != 3 {
		t.Fatalf("got %d, want 3 (only ours removed; malformed + user kept)", len(out))
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
