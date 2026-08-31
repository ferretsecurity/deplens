specification = YAML.load_file("gemspec.yml")

Gem::Specification.new do |spec|
  specification.fetch("development_dependencies").each do |name, versions|
    spec.add_development_dependency(name, versions.split(","))
  end
end
