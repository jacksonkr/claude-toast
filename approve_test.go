package main

import (
	"context"
	"encoding/json"
	"testing"
)

// fakeApprover simulates the relay round-trip without a network. publish records
// the pre-sealed Allow/Deny bodies; subscribe replays one of them (or blocks
// until the deadline) to model a remote tap or silence.
func fakeApprover(respond string, pubErr error) approver {
	var allowBody, denyBody string
	return approver{
		now: func() int64 { return 1000 },
		publish: func(ctx context.Context, server string, req ntfyPublishReq) error {
			for _, a := range req.Actions {
				switch a.Label {
				case "Allow":
					allowBody = a.Body
				case "Deny":
					denyBody = a.Body
				}
			}
			return pubErr
		},
		subscribe: func(ctx context.Context, server string, topics []string, onMessage func(ntfyMessage)) error {
			switch respond {
			case "allow":
				onMessage(ntfyMessage{Message: allowBody})
			case "deny":
				onMessage(ntfyMessage{Message: denyBody})
			default: // "timeout": wait out the context
				<-ctx.Done()
			}
			return nil
		},
	}
}

func approveCfg() config {
	return config{
		Secret:            randSecret(),
		NtfyServer:        "https://ntfy.sh",
		RemoteApprove:     true,
		Broadcast:         true,
		Allowlist:         []string{"Read"},
		ApproveTimeoutSec: 1,
		SummaryCleartext:  true,
		UnreachablePolicy: "deny",
	}
}

func TestDecideOffAllowlistAsks(t *testing.T) {
	cfg := approveCfg()
	p := hookPayload{ToolName: "Bash", ToolInput: json.RawMessage(`{"command":"rm -rf /"}`)}
	d, _ := decidePreToolUse(cfg, p, fakeApprover("allow", nil))
	if d != "ask" {
		t.Errorf("off-allowlist tool: decision = %q, want ask (never remote-approvable)", d)
	}
}

func TestDecideDisabledAsks(t *testing.T) {
	cfg := approveCfg()
	cfg.RemoteApprove = false
	d, _ := decidePreToolUse(cfg, hookPayload{ToolName: "Read"}, fakeApprover("allow", nil))
	if d != "ask" {
		t.Errorf("disabled: decision = %q, want ask", d)
	}
}

func TestDecideRemoteAllow(t *testing.T) {
	cfg := approveCfg()
	p := hookPayload{ToolName: "Read", ToolInput: json.RawMessage(`{"file_path":"/x"}`)}
	d, _ := decidePreToolUse(cfg, p, fakeApprover("allow", nil))
	if d != "allow" {
		t.Errorf("decision = %q, want allow", d)
	}
}

func TestDecideRemoteDeny(t *testing.T) {
	cfg := approveCfg()
	p := hookPayload{ToolName: "Read", ToolInput: json.RawMessage(`{"file_path":"/x"}`)}
	d, _ := decidePreToolUse(cfg, p, fakeApprover("deny", nil))
	if d != "deny" {
		t.Errorf("decision = %q, want deny", d)
	}
}

func TestDecideTimeoutDefaultsDeny(t *testing.T) {
	cfg := approveCfg()
	p := hookPayload{ToolName: "Read", ToolInput: json.RawMessage(`{"file_path":"/x"}`)}
	d, _ := decidePreToolUse(cfg, p, fakeApprover("timeout", nil))
	if d != "deny" {
		t.Errorf("timeout decision = %q, want deny (fail-safe)", d)
	}
}

func TestDecideUnreachableDefaultsDeny(t *testing.T) {
	cfg := approveCfg()
	p := hookPayload{ToolName: "Read", ToolInput: json.RawMessage(`{"file_path":"/x"}`)}
	d, _ := decidePreToolUse(cfg, p, fakeApprover("timeout", context.DeadlineExceeded))
	if d != "deny" {
		t.Errorf("unreachable decision = %q, want deny", d)
	}
}

func TestDecideTimeoutAskPolicy(t *testing.T) {
	cfg := approveCfg()
	cfg.UnreachablePolicy = "ask"
	p := hookPayload{ToolName: "Read", ToolInput: json.RawMessage(`{"file_path":"/x"}`)}
	d, _ := decidePreToolUse(cfg, p, fakeApprover("timeout", nil))
	if d != "ask" {
		t.Errorf("ask-policy timeout decision = %q, want ask", d)
	}
}
