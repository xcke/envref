# typed: false
# frozen_string_literal: true

# Homebrew formula for envref
# This is a template — GoReleaser auto-generates the actual formula
# in the xcke/homebrew-tap repository on each release.
#
# Manual install (from source):
#   brew install --build-from-source Formula/envref.rb
class Envref < Formula
  desc "CLI tool for separating config from secrets in .env files"
  homepage "https://github.com/xcke/envref"
  license "MIT"
  head "https://github.com/xcke/envref.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X github.com/xcke/envref/internal/cmd.version=#{version}
    ]
    system "go", "build", *std_go_args(ldflags:), "./cmd/envref"
  end

  test do
    assert_match "envref", shell_output("#{bin}/envref --help")

    # Verify version flag works
    assert_match version.to_s, shell_output("#{bin}/envref version")
  end
end
