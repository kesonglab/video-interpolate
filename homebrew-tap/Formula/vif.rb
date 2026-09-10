class Vif < Formula
  desc "AI video frame interpolation TUI"
  homepage "https://github.com/kesonglab/video-interpolate"
  url "https://github.com/kesonglab/video-interpolate/releases/download/v0.2.0/vif_0.2.0_darwin_arm64.tar.gz"
  sha256 "94b98801aac373e093fe49076f8ebfdf30a410ce9614b016c4a0abaa4cd13030"
  license "MIT"

  depends_on "ffmpeg"

  def install
    bin.install "vif"
  end

  test do
    assert_match "vif", shell_output("#{bin}/vif version")
  end
end
