Gem::Specification.new do |spec|
  spec.add_dependency "http-client", ENV["HTTP_CLIENT_VERSION"] || ">= 1.10.0"
end
