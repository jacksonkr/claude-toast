#!/usr/bin/env python3
"""Update the Scoop manifest + Homebrew formula for a new release.

Usage: update.py <version> <windows_zip_sha256> <darwin_tgz_sha256> <linux_tgz_sha256>

Run by the release workflow after the binaries are published, so `scoop install`
and `brew install` always point at the latest version with correct hashes.
"""
import json
import re
import sys

ver, win, mac, lin = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
base = "https://github.com/jacksonkr/claude-toast/releases/download"

# --- Scoop manifest ---
sp = "packaging/scoop/claude-toast.json"
with open(sp) as f:
    d = json.load(f)
d["version"] = ver
a = d["architecture"]["64bit"]
a["hash"] = win
a["url"] = f"{base}/v{ver}/claude-toast-windows-amd64.zip"
with open(sp, "w") as f:
    json.dump(d, f, indent=4)
    f.write("\n")

# --- Homebrew formula ---
hp = "packaging/homebrew/claude-toast.rb"
with open(hp) as f:
    s = f.read()
s = re.sub(r'version "[^"]*"', f'version "{ver}"', s, count=1)
s = re.sub(r"(download/v)[^/]+(/claude-toast-darwin-arm64\.tar\.gz)", rf"\g<1>{ver}\g<2>", s)
s = re.sub(r"(download/v)[^/]+(/claude-toast-linux-amd64\.tar\.gz)", rf"\g<1>{ver}\g<2>", s)
s = re.sub(r'(claude-toast-darwin-arm64\.tar\.gz"\n\s*sha256 ")[0-9a-f]{64}(")', rf"\g<1>{mac}\g<2>", s)
s = re.sub(r'(claude-toast-linux-amd64\.tar\.gz"\n\s*sha256 ")[0-9a-f]{64}(")', rf"\g<1>{lin}\g<2>", s)
with open(hp, "w") as f:
    f.write(s)

print(f"updated manifests to v{ver}")
