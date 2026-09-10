class Vif < Formula
  desc "AI video frame interpolation TUI"
  homepage "https://github.com/kesonglab/video-interpolate"
  url "https://github.com/kesonglab/video-interpolate/releases/download/v0.3.0/vif_0.3.0_darwin_arm64.tar.gz"
  sha256 "357b6f83a7bf84ba5d7bf37d34c588d0a5975167065e053e8ed3beae6498c542"
  license "MIT"

  depends_on "ffmpeg"

  def install
    bin.install "vif"
  end

  test do
    assert_match "vif", shell_output("#{bin}/vif version")
  end
end
