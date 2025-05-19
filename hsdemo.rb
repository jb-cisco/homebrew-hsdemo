class Hsdemo < Formula
  desc "Sets up hs demo in eks cluster (DEPRECATED - use jb-cisco/hsd/hsd instead)"
  homepage "https://github.com/jb-cisco/homebrew-hsdemo"
  version "1.9"
  url "https://github.com/jb-cisco/homebrew-hsdemo/releases/download/v1.6/hsdemo"

  # List of dependencies
  depends_on "eksdemo"
  depends_on "awscli"
  depends_on "kubernetes-cli"
  depends_on "helm"
  depends_on :macos

  def caveats
    <<~EOS
      DEPRECATED!
      This formula is deprecated and will be removed in a future release.
      Please use the new formula instead:
        brew install jb-cisco/hsd/hsd
    EOS
  end

  def install
    opoo "This formula is deprecated! Please use 'brew install jb-cisco/hsd/hsd' instead."

    # Check for ARM architecture
    odie "This formula only supports ARM architecture." unless Hardware::CPU.arm?

    # Install the binary
    bin.install "hsdemo"

    # Create a wrapper script that shows the deprecation notice
    (bin/"hsdemo").write <<~EOS
      #!/bin/bash
      echo "WARNING: hsdemo is deprecated. Please use 'brew install jb-cisco/hsd/hsd' instead."
      echo "This wrapper will execute the original binary this time."
      echo ""
      exec "#{bin}/hsdemo.real" "$@"
    EOS

    # Rename the original binary
    mv bin/"hsdemo", bin/"hsdemo.real"
    chmod 0755, bin/"hsdemo"
  end
end
