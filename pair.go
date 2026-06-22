package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// runPair handles `claude-toast pair`:
//
//	pair --server <url> [--force]   first device: generate a secret, print a token
//	pair --join <token>             other devices: adopt the secret from the token
//	pair                            reprint the existing pairing token
//
// The pairing token carries both the ntfy server and the secret:
//
//	claude-toast://pair?s=<server>&k=<secret>
func runPair(args []string) {
	fs := flag.NewFlagSet("pair", flag.ContinueOnError)
	server := fs.String("server", "", "ntfy server base URL, e.g. https://ntfy.example.com")
	join := fs.String("join", "", "pairing token from your first device")
	force := fs.Bool("force", false, "regenerate the secret even if one exists (re-pair all devices)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	switch {
	case *join != "":
		if err := joinFromToken(&cfg, *join); err != nil {
			fmt.Fprintln(os.Stderr, "pair --join:", err)
			os.Exit(2)
		}
		mustSave(cfg)
		fmt.Println("Paired with your other devices. 🍞")
		printSubscribeHint(cfg)

	case *server != "" || (cfg.Secret != "" && !*force):
		// Generate (first device) or reprint (already paired).
		if *server != "" {
			cfg.NtfyServer = strings.TrimRight(*server, "/")
		}
		if cfg.NtfyServer == "" {
			fmt.Fprintln(os.Stderr, "pair: --server <url> is required for the first device")
			os.Exit(2)
		}
		if cfg.Secret == "" || *force {
			cfg.Secret = randSecret()
			cfg.Broadcast = true
		}
		ensureIdentity(&cfg)
		mustSave(cfg)
		printPairing(cfg)

	default:
		fmt.Fprintln(os.Stderr, "pair: provide --server <url> (first device) or --join <token> (other devices)")
		os.Exit(2)
	}
}

// joinFromToken parses a pairing token and adopts its server + secret, taking on
// a fresh local identity.
func joinFromToken(cfg *config, token string) error {
	u, err := url.Parse(strings.TrimSpace(token))
	if err != nil || u.Scheme != "claude-toast" {
		return fmt.Errorf("invalid token (expected claude-toast://pair?...)")
	}
	q := u.Query()
	s, k := q.Get("s"), q.Get("k")
	if s == "" || k == "" {
		return fmt.Errorf("token missing server or key")
	}
	b, err := base64.RawURLEncoding.DecodeString(k)
	if err != nil || len(b) != 32 {
		return fmt.Errorf("token key is malformed")
	}
	cfg.NtfyServer = strings.TrimRight(s, "/")
	cfg.Secret = k
	cfg.Broadcast = true
	cfg.DeviceID = "" // force a fresh identity for this device
	ensureIdentity(cfg)
	return nil
}

// initIdentity makes the device a ready-to-run node: it fills in the relay, a
// fresh UID (the linking secret), and a local identity if any are missing. It
// reports whether it changed anything (so callers can decide to save). This is
// what makes claude-toast "just run" with a UID after install, with no explicit
// pairing step — you only act when you want to LINK two devices.
func initIdentity(cfg *config) bool {
	changed := false
	if cfg.NtfyServer == "" {
		cfg.NtfyServer = defaultServer
		changed = true
	}
	if cfg.Secret == "" {
		cfg.Secret = randSecret()
		cfg.Broadcast = true
		changed = true
	}
	if cfg.DeviceID == "" || cfg.DeviceName == "" {
		ensureIdentity(cfg)
		changed = true
	}
	return changed
}

// ensureInitialized loads the config, initializes a UID/identity if needed,
// persists any change, and returns the ready config.
func ensureInitialized() config {
	cfg, _ := loadConfig()
	if initIdentity(&cfg) {
		_ = saveConfig(cfg)
	}
	return cfg
}

// ensureIdentity fills in a stable device id and a display name if missing.
func ensureIdentity(cfg *config) {
	if cfg.DeviceID == "" {
		cfg.DeviceID = randHex(16)
	}
	if cfg.DeviceName == "" {
		if name := computerName(); name != "" {
			cfg.DeviceName = name
		} else {
			cfg.DeviceName = "device-" + originID(cfg.DeviceID)
		}
	}
}

// computerName is the friendly device name shown in broadcast toasts (the
// "@name" suffix). On macOS os.Hostname() is usually a transient DHCP/.local
// name (e.g. "Mac.lan"), so we prefer the user-set ComputerName (the "name your
// Mac" field most people recognize), then LocalHostName, and only then fall back
// to os.Hostname(). Elsewhere os.Hostname() is the right answer.
func computerName() string {
	if runtime.GOOS == "darwin" {
		for _, key := range []string{"ComputerName", "LocalHostName"} {
			out, err := exec.Command("scutil", "--get", key).Output()
			if err != nil {
				continue
			}
			if name := strings.TrimSpace(string(out)); name != "" {
				return name
			}
		}
	}
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return ""
}

func pairingToken(cfg config) string {
	u := url.URL{Scheme: "claude-toast", Host: "pair"}
	q := url.Values{}
	q.Set("s", cfg.NtfyServer)
	q.Set("k", cfg.Secret)
	u.RawQuery = q.Encode()
	return u.String()
}

func printPairing(cfg config) {
	fmt.Println("Pairing token (copy onto your other computers, run `claude-toast pair --join <token>`):")
	fmt.Println()
	fmt.Println("    " + pairingToken(cfg))
	fmt.Println()
	fmt.Println("Keep it secret — it controls who can receive (and answer) your toasts.")
	printSubscribeHint(cfg)
}

// printSubscribeHint tells the user what to subscribe to in the ntfy phone app.
func printSubscribeHint(cfg config) {
	ks, ok := keysetFor(cfg)
	if !ok {
		return
	}
	fmt.Println()
	fmt.Println("On your phone: install the ntfy app, set the server to")
	fmt.Println("    " + cfg.NtfyServer)
	fmt.Println("and subscribe to these topics:")
	fmt.Println("    " + ks.broadcastTopic() + "   (toasts)")
	fmt.Println("    " + ks.approveReqTopic() + "   (Allow/Deny prompts, when remote approve is on)")
}

// runUID prints this device's UID (the linking secret). Putting that UID on
// another computer (`claude-toast link <uid>`) joins the two into one group.
func runUID() {
	cfg := ensureInitialized()
	ks, _ := keysetFor(cfg)
	fmt.Println("This device's claude-toast UID:")
	fmt.Println()
	fmt.Println("    " + cfg.Secret)
	fmt.Println()
	fmt.Println("Link another computer to it:   claude-toast link " + cfg.Secret)
	if cfg.NtfyServer != defaultServer {
		fmt.Println("(custom relay: " + cfg.NtfyServer + " — use `claude-toast pair` to share a full token)")
	}
	fmt.Println()
	fmt.Println("Phone (viewer): ntfy app -> server " + cfg.NtfyServer + " -> subscribe to topic:")
	fmt.Println("    " + ks.broadcastTopic())
}

// runLink adopts another device's UID (or a full pairing token for a custom
// relay), joining this device into that group.
func runLink(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "link: provide a UID (or pairing token) from another device")
		os.Exit(2)
	}
	arg := strings.TrimSpace(args[0])
	cfg, _ := loadConfig()

	if strings.HasPrefix(arg, "claude-toast://") {
		if err := joinFromToken(&cfg, arg); err != nil {
			fmt.Fprintln(os.Stderr, "link:", err)
			os.Exit(2)
		}
	} else {
		b, err := base64.RawURLEncoding.DecodeString(arg)
		if err != nil || len(b) != 32 {
			fmt.Fprintln(os.Stderr, "link: that does not look like a claude-toast UID")
			os.Exit(2)
		}
		if cfg.NtfyServer == "" {
			cfg.NtfyServer = defaultServer
		}
		cfg.Secret = arg
		cfg.Broadcast = true
		cfg.DeviceID = "" // take a fresh local identity
		ensureIdentity(&cfg)
	}
	mustSave(cfg)
	fmt.Println("Linked. 🍞 This device now shares toasts with that group.")
	printSubscribeHint(cfg)
}

// pairingInfoText renders the pairing token + phone-subscribe hint as plain
// text, for the tray to drop into a file the user can read/copy.
func pairingInfoText(cfg config) string {
	var b strings.Builder
	if !cfg.paired() {
		b.WriteString("Claude Toast — not paired yet.\n\n")
		b.WriteString("In a terminal, run:\n")
		b.WriteString("    claude-toast pair --server https://your-ntfy-server\n")
		return b.String()
	}
	b.WriteString("Claude Toast — pairing\n\n")
	b.WriteString("Token (run `claude-toast pair --join <token>` on your other computers):\n\n")
	b.WriteString("    " + pairingToken(cfg) + "\n\n")
	if ks, ok := keysetFor(cfg); ok {
		b.WriteString("Phone: open the ntfy app, set server to\n")
		b.WriteString("    " + cfg.NtfyServer + "\n")
		b.WriteString("and subscribe to topic:\n")
		b.WriteString("    " + ks.broadcastTopic() + "\n")
	}
	b.WriteString("\nKeep this token secret.\n")
	return b.String()
}

// showPairingCode writes the pairing info to a file and opens it (used by the
// GUI tray, which has no console to print to).
func showPairingCode() {
	cfg, _ := loadConfig()
	d, err := ensureConfigDir()
	if err != nil {
		return
	}
	p := filepath.Join(d, "pairing.txt")
	if err := os.WriteFile(p, []byte(pairingInfoText(cfg)), 0o600); err != nil {
		return
	}
	_ = openPath(p)
}

// runStatus prints the current cross-device state.
func runStatus() {
	cfg := ensureInitialized()
	ks, _ := keysetFor(cfg)
	fmt.Println("Linked group. 🍞")
	fmt.Println("  UID:             " + cfg.Secret)
	fmt.Println("  relay:           " + cfg.NtfyServer)
	fmt.Println("  this device:     " + cfg.DeviceName + " (" + originID(cfg.DeviceID) + ")")
	fmt.Println("  group fp:        " + cfg.fingerprint() + "   (matches on every linked device)")
	fmt.Println("  broadcast:       " + onoff(cfg.Broadcast))
	fmt.Println("  remote approve:  " + onoff(cfg.RemoteApprove))
	fmt.Println("  broadcast topic: " + ks.broadcastTopic())
}

func onoff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func mustSave(cfg config) {
	if err := saveConfig(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error saving config:", err)
		os.Exit(1)
	}
}
