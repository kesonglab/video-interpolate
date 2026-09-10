class Vif < Formula
  desc "AI video frame interpolation TUI"
  homepage "https://github.com/kesonglab/video-interpolate"
  url "https://github.com/kesonglab/video-interpolate/releases/download/v0.1.0/vif_0.1.0_darwin_arm64.tar.gz"
  sha256 "REPLACE_WITH_ACTUAL_SHA256"
  license "MIT"

  depends_on "ffmpeg"

  def install
    bin.install "vif"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/vif version")
  end
end