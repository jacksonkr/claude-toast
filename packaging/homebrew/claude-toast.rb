class ClaudeToast < Formula
  desc "Desktop toast notifications for Claude Code, with cross-device broadcast + remote approve"
  homepage "https://github.com/jacksonkr/claude-toast"
  version "0.2.0"
  license "MIT"

  # Apple Silicon build. (Intel macs aren't published yet — build from source.)
  on_macos do
    url "https://github.com/jacksonkr/claude-toast/releases/download/v0.2.0/claude-toast-darwin-arm64.tar.gz"
    sha256 "f186967c42089fd221891c1702789a8954c7610ce3a912da90d893d5c4c9d213"
  end

  on_linux do
    url "https://github.com/jacksonkr/claude-toast/releases/download/v0.2.0/claude-toast-linux-amd64.tar.gz"
    sha256 "89622558af70b91067847113f9bb3e42d0f434a1cb7edbd1805eb9b720590ed5"
  end

  def install
    bin.install "claude-toast"
  end

  def caveats
    <<~EOS
      Run `claude-toast install` once to wire the Claude Code hooks + tray autostart,
      then `claude-toast test` to fire a sample toast.
    EOS
  end

  test do
    assert_match "claude-toast", shell_output("#{bin}/claude-toast --help")
  end
end
