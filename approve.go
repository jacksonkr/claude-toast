package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// approver carries the transport + clock as seams so the decision logic is
// unit-testable without a network or wall clock.
type approver struct {
	publish   func(ctx context.Context, server string, req ntfyPublishReq) error
	subscribe func(ctx context.Context, server string, topics []string, onMessage func(ntfyMessage)) error
	now       func() int64
}

func liveApprover() approver {
	return approver{
		publish:   ntfyPublish,
		subscribe: ntfySubscribe,
		now:       func() int64 { return time.Now().Unix() },
	}
}

// decidePreToolUse decides a PreToolUse permission prompt, possibly by asking a
// remote device. It returns a permissionDecision — "allow", "deny", or "ask" —
// and a human reason. "ask" means "fall through to Claude's normal local
// prompt"; we use it whenever remote approval doesn't apply, so the user at the
// keyboard is never worse off.
//
// Authority is deny-only with a safe allowlist: only allowlisted tools are ever
// sent for remote approval, and silence (timeout / unreachable) defaults to deny.
func decidePreToolUse(cfg config, p hookPayload, ap approver) (string, string) {
	if !cfg.RemoteApprove || !cfg.paired() {
		return "ask", ""
	}
	if isPaused() {
		return "ask", "claude-toast paused"
	}
	if !toolAllowed(cfg, p.ToolName) {
		// Deny-only: off-allowlist tools can never be remote-approved, so don't
		// even ask — let the human at the origin decide locally.
		return "ask", "not remotely approvable"
	}
	ks, ok := keysetFor(cfg)
	if !ok {
		return "ask", ""
	}

	now := ap.now()
	req := approveReq{
		Type:      typeApproveReq,
		Origin:    cfg.DeviceID,
		OriginNm:  cfg.DeviceName,
		Nonce:     randHex(16),
		Tool:      p.ToolName,
		Summary:   summarizeTool(p.ToolName, p.ToolInput),
		InputHash: inputHash(p.ToolInput),
		Project:   titleFromCwd(p.CWD),
		TS:        now,
		ExpiresTS: now + int64(cfg.ApproveTimeoutSec),
	}

	// Pre-seal both outcomes so a remote device (which holds no key) only picks
	// which authenticated, nonce-bound blob to send back — it can't forge a
	// decision or replay one onto a different request.
	allowEnv, err1 := sealJSON(ks, approveResp{Type: typeApproveResp, Nonce: req.Nonce, Decision: "allow", TS: now})
	denyEnv, err2 := sealJSON(ks, approveResp{Type: typeApproveResp, Nonce: req.Nonce, Decision: "deny", TS: now})
	if err1 != nil || err2 != nil {
		return cfg.unreachable()
	}
	allowBody, _ := json.Marshal(allowEnv)
	denyBody, _ := json.Marshal(denyEnv)
	respURL := strings.TrimRight(cfg.NtfyServer, "/") + "/" + ks.approveRespTopic()

	title := "Claude needs permission"
	msg := "Approve a tool? Open to decide."
	if cfg.SummaryCleartext {
		title = "Approve " + req.Tool + " in " + req.Project + "?"
		msg = req.Summary
	}

	pubReq := ntfyPublishReq{
		Topic:   ks.approveReqTopic(),
		Title:   title,
		Message: msg,
		Tags:    []string{originTag(cfg.DeviceID)},
		Actions: []ntfyAction{
			{Action: "http", Label: "Allow", URL: respURL, Method: "POST", Body: string(allowBody), Clear: true},
			{Action: "http", Label: "Deny", URL: respURL, Method: "POST", Body: string(denyBody), Clear: true},
		},
	}

	// One deadline governs the whole exchange so we always return before Claude
	// kills the hook at its configured timeout.
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ApproveTimeoutSec)*time.Second)
	defer cancel()

	if err := ap.publish(ctx, cfg.NtfyServer, pubReq); err != nil {
		return cfg.unreachable()
	}

	decision := waitForDecision(ctx, ap, cfg, ks, req)
	switch decision {
	case "allow":
		return "allow", "approved remotely"
	case "deny":
		return "deny", "denied remotely"
	default:
		return cfg.unreachable()
	}
}

// waitForDecision blocks on the response topic until a valid, nonce-matched,
// unexpired decision arrives or ctx ends. Returns "" if none.
func waitForDecision(ctx context.Context, ap approver, cfg config, ks keyset, req approveReq) string {
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var decision string
	_ = ap.subscribe(subCtx, cfg.NtfyServer, []string{ks.approveRespTopic()}, func(m ntfyMessage) {
		if decision != "" {
			return
		}
		var env envelope
		if json.Unmarshal([]byte(m.Message), &env) != nil {
			return
		}
		pt, ok := open(ks, env)
		if !ok {
			return
		}
		var r approveResp
		if json.Unmarshal(pt, &r) != nil || r.Type != typeApproveResp {
			return
		}
		if r.Nonce != req.Nonce || ap.now() > req.ExpiresTS {
			return
		}
		if r.Decision == "allow" || r.Decision == "deny" {
			decision = r.Decision
			cancel() // stop the subscription
		}
	})
	return decision
}

func (c config) unreachable() (string, string) {
	if c.UnreachablePolicy == "ask" {
		return "ask", "no remote response"
	}
	return "deny", "no remote response (default deny)"
}

// setRemoteApprove flips the remote-approve mode, persists it, and rewrites the
// hooks (adding/removing the PreToolUse hook). Returns true on success. Used by
// the tray toggle.
func setRemoteApprove(enabled bool) bool {
	cfg, err := loadConfig()
	if err != nil {
		return false
	}
	cfg.RemoteApprove = enabled
	if err := saveConfig(cfg); err != nil {
		return false
	}
	return reconcileHooks(cfg) == nil
}

// runRemote toggles remote-approve from the CLI (parity with the tray
// checkbox): it persists the setting and rewires the PreToolUse hook.
func runRemote(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: claude-toast remote <on|off>")
		os.Exit(2)
	}
	var on bool
	switch strings.ToLower(args[0]) {
	case "on", "enable", "true":
		on = true
	case "off", "disable", "false":
		on = false
	default:
		fmt.Fprintln(os.Stderr, "usage: claude-toast remote <on|off>")
		os.Exit(2)
	}
	if !setRemoteApprove(on) {
		fmt.Fprintln(os.Stderr, "failed to update remote approve")
		os.Exit(1)
	}
	if !on {
		fmt.Println("Remote approve: OFF")
		return
	}
	cfg, _ := loadConfig()
	fmt.Println("Remote approve: ON")
	fmt.Println("  allowed tools: " + strings.Join(cfg.Allowlist, ", "))
	if ks, ok := keysetFor(cfg); ok {
		fmt.Println("  phone: also subscribe to the approval topic:")
		fmt.Println("    " + ks.approveReqTopic())
	}
	fmt.Println("  note: Claude will ask your phone before these tools and block up to")
	fmt.Printf("        %ds (deny on no answer). Turn off with `claude-toast remote off`.\n", cfg.ApproveTimeoutSec)
}

// runSimulatePreToolUse exercises the full publish->wait->decide path from the
// CLI (no Claude needed), printing the decision JSON and elapsed time.
func runSimulatePreToolUse(args []string) {
	fs := flag.NewFlagSet("simulate-pretooluse", flag.ContinueOnError)
	tool := fs.String("tool", "Read", "tool name")
	input := fs.String("input", "{}", "tool_input JSON")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	cwd, _ := os.Getwd()
	cfg, _ := loadConfig()
	p := hookPayload{
		HookEventName: "PreToolUse",
		CWD:           cwd,
		ToolName:      *tool,
		ToolInput:     json.RawMessage(*input),
	}
	start := time.Now()
	decision, reason := decidePreToolUse(cfg, p, liveApprover())
	emitPreToolUse(decision, reason)
	fmt.Fprintf(os.Stderr, "decision=%s reason=%q elapsed=%s\n", decision, reason, time.Since(start).Round(time.Millisecond))
}

func toolAllowed(cfg config, tool string) bool {
	for _, t := range cfg.Allowlist {
		if t == tool {
			return true
		}
	}
	return false
}

// inputHash is a stable digest of the tool input, included in the request for
// audit/context. Canonicalize via map round-trip (Go marshals map keys sorted).
func inputHash(raw json.RawMessage) string {
	var v any
	if len(raw) > 0 && json.Unmarshal(raw, &v) == nil {
		if b, err := json.Marshal(v); err == nil {
			sum := sha256.Sum256(b)
			return hex.EncodeToString(sum[:])[:16]
		}
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:16]
}

// summarizeTool builds a short human description like "Read /path" for the
// remote notification.
func summarizeTool(tool string, raw json.RawMessage) string {
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	for _, k := range []string{"file_path", "path", "pattern", "command", "url", "query"} {
		if s, ok := m[k].(string); ok && s != "" {
			if len(s) > 80 {
				s = s[:77] + "..."
			}
			return tool + " " + s
		}
	}
	return tool
}
