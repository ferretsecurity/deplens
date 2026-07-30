Gem::Specification.new do |spec|
  spec.name = "manual-dependencies"
  spec.version = "1.0.0"
  spec.summary = "Direct Gem Dependency declarations"
  spec.authors = ["Fixture Author"]
  spec.files = []

  spec.dependencies << Gem::Dependency.new("manual-runtime", ">= 1.0", :runtime)
  spec.dependencies << Gem::Dependency.new("manual-development", "~> 2.0", :development)
end
