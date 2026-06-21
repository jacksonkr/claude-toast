package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// config is claude-toast's cross-device settings, stored as JSON in the config
// dir alongside the pause flag and extracted icon. It holds the pairing secret,
// so the file is written mode 0600. A missing file yields a zero config in which
// nothing networked happens (Broadcast/RemoteApprove false) until the user pairs.
type config struct {
	Version           int      `json:"version"`                   // schema version (1)
	DeviceID          string   `json:"device_id"`                 // 16-byte hex, stable per install
	DeviceName        string   `json:"device_name"`               // os.Hostname() default, for display
	NtfyServer        string   `json:"ntfy_server"`               // base URL, no trailing slash
	Secret            string   `json:"secret"`                    // base64url of 32 random bytes
	Broadcast         bool     `json:"broadcast"`                 // publish Notification/Stop to peers
	RemoteApprove     bool     `json:"remote_approve"`            // enable PreToolUse remote decisions
	Allowlist         []string `json:"allowlist"`                 // tools that MAY be remotely approved
	ApproveTimeoutSec int      `json:"approve_timeout_sec"`       // internal wait (must be < hook timeout)
	SummaryCleartext  bool     `json:"approve_summary_cleartext"` // readable phone summary
	UnreachablePolicy string   `json:"approve_unreachable"`       // "deny" | "ask"
}

func configPath() string { return filepath.Join(configDir(), "config.json") }

// defaultServer is the relay every device dials out to. Public ntfy.sh needs no
// setup; our payloads are end-to-end ready and topics are unguessable, and the
// device only ever makes OUTBOUND connections (it never listens for a peer), so
// no machine exposes an inbound port.
const defaultServer = "https://ntfy.sh"

// defaultConfig returns a config pre-populated with defaults. It is the base
// onto which an existing file is unmarshaled, so missing fields inherit them.
func defaultConfig() config {
	return config{
		Version:           1,
		NtfyServer:        defaultServer,
		Broadcast:         true,
		Allowlist:         []string{"Read", "Glob", "Grep", "LS"},
		ApproveTimeoutSec: 12,
		SummaryCleartext:  true,
		UnreachablePolicy: "deny",
	}
}

// loadConfig reads config.json. A missing file is not an error: it returns the
// defaults (unpaired, so every networked path is a no-op).
func loadConfig() (config, error) {
	c := defaultConfig()
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("parsing %s: %w", configPath(), err)
	}
	return c, nil
}

// saveConfig writes config.json atomically (temp + rename) at mode 0600 so the
// tray never reads a torn file while the hook writes, and the secret stays
// owner-only.
func saveConfig(c config) error {
	d, err := ensureConfigDir()
	if err != nil {
		return err
	}
	out, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	tmp := filepath.Join(d, "config.json.tmp")
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, configPath())
}

// paired reports whether the device has a secret and a server, i.e. is ready to
// talk to peers.
func (c config) paired() bool { return c.Secret != "" && c.NtfyServer != "" }

// secretBytes decodes the pairing secret to raw bytes.
func (c config) secretBytes() ([]byte, bool) {
	if c.Secret == "" {
		return nil, false
	}
	b, err := base64.RawURLEncoding.DecodeString(c.Secret)
	if err != nil || len(b) != 32 {
		return nil, false
	}
	return b, true
}

// fingerprint is a short, non-reversible label for the secret, shown by `status`
// so two devices can confirm they share the same secret without printing it.
func (c config) fingerprint() string {
	b, ok := c.secretBytes()
	if !ok {
		return "(none)"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:6]
}

func randSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
