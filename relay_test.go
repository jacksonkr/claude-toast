package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func testSecret() []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(i + 1)
	}
	return b
}

func TestDeriveKeysDeterministic(t *testing.T) {
	a := deriveKeys(testSecret())
	b := deriveKeys(testSecret())
	if a.key != b.key {
		t.Error("same secret yielded different keys")
	}
	if a.topicBase != b.topicBase || a.topicBase == "" {
		t.Errorf("topic base not stable/nonempty: %q vs %q", a.topicBase, b.topicBase)
	}
	if a.broadcastTopic() == a.approveReqTopic() || a.broadcastTopic() == a.approveRespTopic() {
		t.Error("topics are not distinct")
	}
	// Different secret -> different derivation.
	other := testSecret()
	other[0] ^= 0xff
	if deriveKeys(other).key == a.key {
		t.Error("different secret produced same key")
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	ks := deriveKeys(testSecret())
	msg := []byte(`{"hello":"world"}`)
	env, err := seal(ks, msg)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	got, ok := open(ks, env)
	if !ok {
		t.Fatal("open failed on a valid envelope")
	}
	if !bytes.Equal(got, msg) {
		t.Errorf("round-trip mismatch: %q != %q", got, msg)
	}
}

func TestOpenRejectsTamperAndWrongKey(t *testing.T) {
	ks := deriveKeys(testSecret())
	env, _ := seal(ks, []byte("secret payload"))

	// Tamper with the ciphertext.
	bad := env
	bad.CT = bad.CT[:len(bad.CT)-2] + "AA"
	if _, ok := open(ks, bad); ok {
		t.Error("open accepted tampered ciphertext")
	}

	// Wrong key.
	other := testSecret()
	other[5] ^= 0x01
	if _, ok := open(deriveKeys(other), env); ok {
		t.Error("open accepted envelope under the wrong key")
	}

	// Wrong version.
	wrongV := env
	wrongV.V = 2
	if _, ok := open(ks, wrongV); ok {
		t.Error("open accepted unknown envelope version")
	}
}

func TestConfigDefaultsMergedOverPartialFile(t *testing.T) {
	// loadConfig unmarshals the file over defaultConfig(); a partial file must
	// inherit the defaults it omits.
	c := defaultConfig()
	if err := json.Unmarshal([]byte(`{"ntfy_server":"https://n.example.com"}`), &c); err != nil {
		t.Fatal(err)
	}
	if c.NtfyServer != "https://n.example.com" {
		t.Error("explicit field not applied")
	}
	if len(c.Allowlist) != 4 {
		t.Errorf("default allowlist lost: %v", c.Allowlist)
	}
	if c.ApproveTimeoutSec != 18 || !c.SummaryCleartext || c.UnreachablePolicy != "deny" {
		t.Error("scalar defaults not inherited")
	}
}

func TestPairingTokenRoundTrip(t *testing.T) {
	src := config{NtfyServer: "https://ntfy.example.com", Secret: randSecret()}
	token := pairingToken(src)

	var got config
	if err := joinFromToken(&got, token); err != nil {
		t.Fatalf("joinFromToken: %v", err)
	}
	if got.NtfyServer != src.NtfyServer {
		t.Errorf("server = %q, want %q", got.NtfyServer, src.NtfyServer)
	}
	if got.Secret != src.Secret {
		t.Error("secret did not survive the token round-trip")
	}
	if got.DeviceID == "" || !got.Broadcast {
		t.Error("join did not establish a local identity / enable broadcast")
	}
}

func TestJoinRejectsBadToken(t *testing.T) {
	var c config
	for _, bad := range []string{
		"http://wrong-scheme",
		"claude-toast://pair?s=https://x", // missing key
		"claude-toast://pair?k=abc",       // missing server
		"not a url at all %%%",
	} {
		if err := joinFromToken(&c, bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestBroadcastEchoSuppression(t *testing.T) {
	cfg := config{DeviceID: "0123456789abcdef0011", DeviceName: "me"}

	// Our own broadcast echoed back -> skip.
	self := ntfyMessage{Title: "proj", Message: "x", Tags: []string{originTag(cfg.DeviceID)}}
	if _, ok := broadcastToNotification(cfg, self); ok {
		t.Error("did not suppress our own echoed broadcast")
	}

	// From another device -> show.
	other := ntfyMessage{Title: "proj @other", Message: "line1\nline2", Tags: []string{originTag("ffffffffffff0000")}}
	n, ok := broadcastToNotification(cfg, other)
	if !ok {
		t.Fatal("suppressed a peer's broadcast")
	}
	if n.Title != "proj @other" || len(n.Lines) != 2 {
		t.Errorf("unexpected notification: %+v", n)
	}
}
