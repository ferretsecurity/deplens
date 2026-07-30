Gem::Specification.new do |spec|
  spec.name = "multiple-constraints"
  spec.version = "1.0.0"
  spec.summary = "Multiple version requirements"
  spec.authors = ["Fixture Author"]
  spec.files = []

  spec.add_runtime_dependency "bounded-kit", ">= 2.0", "< 4.0", "!= 2.2.1"
  spec.add_dependency "prerelease-kit", ">= 3.0.0.a", "< 3.0.0"
end
