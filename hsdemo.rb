class Hsdemo < Formula
    desc "Sets up hs demo in eks cluster"
    version "1.5"
    url "https://github.com/jb-cisco/homebrew-hsdemo/releases/download/v1.5/hsdemo" 
    # List of dependencies
  depends_on "eksdemo"
  depends_on "awscli"
  depends_on "kubernetes-cli"
  depends_on "helm"
  depends_on :macos

  def install
    odie "This formula only supports ARM architecture." unless Hardware::CPU.arm?
    # Proceed with installation
  end
  
  def install
    bin.install "hsdemo"
  end
end