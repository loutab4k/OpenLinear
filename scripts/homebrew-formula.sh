#!/bin/sh
# Generate the Homebrew formula for a released version.
# Usage: scripts/homebrew-formula.sh <version-tag> <checksums.txt> > openlinear.rb
set -eu

TAG="$1"
CHECKSUMS="$2"
BASE="https://github.com/loutab4k/OpenLinear/releases/download/${TAG}"

sum() {
  awk -v name="openlinear-${TAG}-$1" '$2 == name {print $1}' "$CHECKSUMS"
}

cat <<EOF
class Openlinear < Formula
  desc "Telegram-native project tracker rendered as one editable rich message"
  homepage "https://github.com/loutab4k/OpenLinear"
  version "${TAG#v}"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "${BASE}/openlinear-${TAG}-darwin-arm64"
      sha256 "$(sum darwin-arm64)"
    else
      url "${BASE}/openlinear-${TAG}-darwin-amd64"
      sha256 "$(sum darwin-amd64)"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "${BASE}/openlinear-${TAG}-linux-arm64"
      sha256 "$(sum linux-arm64)"
    else
      url "${BASE}/openlinear-${TAG}-linux-amd64"
      sha256 "$(sum linux-amd64)"
    end
  end

  def install
    bin.install Dir["openlinear-*"].first => "ol"
    bin.install_symlink "ol" => "openlinear"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/ol version")
  end
end
EOF
