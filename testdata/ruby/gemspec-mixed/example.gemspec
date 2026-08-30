Gem::Specification.new do |spec|
  spec.add_dependency "shared-library"

  inherited_dependencies.each do |name, requirements|
    spec.add_development_dependency(name, *requirements)
  end
end
