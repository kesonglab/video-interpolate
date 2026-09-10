class Vif < Formula
  desc "AI video frame interpolation TUI"
  homepage "https://github.com/kesonglab/video-interpolate"
  url "https://github.com/kesonglab/video-interpolate/releases/download/v0.1.0/vif_0.1.0_darwin_arm64.tar.gz"
  sha256 "697b5354a4bcc65f09f843f57bdc21e9671530ec3b053204dc5de67e497c6052"
  license "MIT"

  depends_on "ffmpeg"

  def install
    bin.install "vif"
  end

  test do
    assert_match "vif", shell_output("#{bin}/vif version")
  end
end
