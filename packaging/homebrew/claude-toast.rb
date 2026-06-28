class ClaudeToast < Formula
  desc "Desktop toast notifications for Claude Code, with cross-device broadcast + remote approve"
  homepage "https://github.com/jacksonkr/claude-toast"
  version "0.2.1"
  license "MIT"

  # Apple Silicon build. (Intel macs aren't published yet — build from source.)
  on_macos do
    url "https://github.com/jacksonkr/claude-toast/releases/download/v0.2.1/claude-toast-darwin-arm64.tar.gz"
    sha256 "b0cc8fe692c29b413af825aeec501adb074e353646bb20badbc1724a390ba7d5"
  end

  on_linux do
    url "https://github.com/jacksonkr/claude-toast/releases/download/v0.2.1/claude-toast-linux-amd64.tar.gz"
    sha256 "55dcbc99820a05c59ce5490099d01b5989daa3be9a551af8fe191e52bb382a25"
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
