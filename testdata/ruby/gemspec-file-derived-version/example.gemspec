version = File.read("VERSION").strip

Gem::Specification.new do |spec|
  spec.add_dependency "framework-core", version
  spec.add_dependency "request-router", ">= 1.8.5"
end
