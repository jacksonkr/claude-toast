package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"

	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/nacl/secretbox"
)

// hkdfInfo namespaces the key derivation so the same secret used elsewhere would
// not yield the same key/topics. Bump with any breaking wire change.
const hkdfInfo = "claude-toast/v1"

// keyset is everything derived deterministically from the pairing secret. The
// ntfy server only ever sees the opaque topic names and ciphertext.
type keyset struct {
	key       [32]byte // secretbox key
	topicBase string   // unguessable base for the three topics
}

func (k keyset) broadcastTopic() string { return k.topicBase + "-bc" }
func (k keyset) approveReqTopic() string { return k.topicBase + "-aq" }
func (k keyset) approveRespTopic() string { return k.topicBase + "-ar" }

// keysetFor derives the keyset for a config, or ok=false if it has no valid
// secret.
func keysetFor(c config) (keyset, bool) {
	b, ok := c.secretBytes()
	if !ok {
		return keyset{}, false
	}
	return deriveKeys(b), true
}

// deriveKeys turns the 32-byte pairing secret into the AEAD key and the topic
// base, via HKDF-SHA256. Same secret in -> same keyset out, on every device.
func deriveKeys(secret []byte) keyset {
	h := hkdf.New(sha256.New, secret, nil, []byte(hkdfInfo))
	var ks keyset
	_, _ = io.ReadFull(h, ks.key[:])
	var tb [16]byte
	_, _ = io.ReadFull(h, tb[:])
	ks.topicBase = strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(tb[:]))
	return ks
}

// envelope is the JSON written as the ntfy message body: a versioned wrapper
// around a per-message nonce and the secretbox ciphertext.
type envelope struct {
	V  int    `json:"v"`  // envelope version (1)
	N  string `json:"n"`  // base64 nonce (24 bytes)
	CT string `json:"ct"` // base64 ciphertext
}

// seal encrypts+authenticates plaintext under the keyset with a fresh random
// nonce.
func seal(ks keyset, plaintext []byte) (envelope, error) {
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return envelope{}, err
	}
	ct := secretbox.Seal(nil, plaintext, &nonce, &ks.key)
	return envelope{
		V:  1,
		N:  base64.StdEncoding.EncodeToString(nonce[:]),
		CT: base64.StdEncoding.EncodeToString(ct),
	}, nil
}

// open decrypts an envelope. It returns ok=false (and no error) on any auth
// failure or malformed input, so callers can silently drop forged/garbage
// messages.
func open(ks keyset, e envelope) ([]byte, bool) {
	if e.V != 1 {
		return nil, false
	}
	nb, err := base64.StdEncoding.DecodeString(e.N)
	if err != nil || len(nb) != 24 {
		return nil, false
	}
	ct, err := base64.StdEncoding.DecodeString(e.CT)
	if err != nil {
		return nil, false
	}
	var nonce [24]byte
	copy(nonce[:], nb)
	return secretbox.Open(nil, ct, &nonce, &ks.key)
}

// Inner payloads. All carry a Type discriminator so one subscription can route
// broadcast vs approve messages.

const (
	typeBroadcast   = "bc"
	typeApproveReq  = "aq"
	typeApproveResp = "ar"
)

type bcPayload struct {
	Type     string   `json:"t"`
	Origin   string   `json:"o"`  // sender DeviceID, for echo suppression
	OriginNm string   `json:"on"` // sender DeviceName, for display
	Event    string   `json:"e"`  // Notification | Stop
	Title    string   `json:"ti"`
	Lines    []string `json:"l"`
	TS       int64    `json:"ts"`
}

type approveReq struct {
	Type      string `json:"t"`
	Origin    string `json:"o"`
	OriginNm  string `json:"on"`
	Nonce     string `json:"n"`  // 16-byte hex correlation id (distinct from AEAD nonce)
	Tool      string `json:"to"` // tool name
	Summary   string `json:"s"`  // human-readable, e.g. "Read /path"
	InputHash string `json:"ih"` // sha256 of canonical tool_input
	Project   string `json:"p"`  // last path segment of cwd
	TS        int64  `json:"ts"`
	ExpiresTS int64  `json:"ex"`
}

type approveResp struct {
	Type      string `json:"t"`
	Nonce     string `json:"n"`
	Decision  string `json:"d"` // "allow" | "deny"
	Responder string `json:"r"` // DeviceName of responder, if known
	TS        int64  `json:"ts"`
}

// sealJSON marshals v and seals it in one step.
func sealJSON(ks keyset, v any) (envelope, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return envelope{}, err
	}
	return seal(ks, b)
}
