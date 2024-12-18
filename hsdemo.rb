class Hsdemo < Formula
    desc "Sets up hs demo in eks cluster"
    url "https://example.com/my_package-1.0.0.tar.gz"

    # List of dependencies
  depends_on "eksdemo"

  
    def install
      # Prompt for user input
      puts "Please enter something:"
      package_name = gets.chomp
  
  
  
      # Example command to simulate installation
      system "echo got #{package_name}"
  
      # Here you would typically have the actual installation commands
      # For example:
      # system "brew install #{package_name} --version=#{version}"
  
      puts "#{package_name} version #{version} installed successfully!"
    end
  end