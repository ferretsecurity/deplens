dependencies = {}

Gem::Specification.new do |spec|
  dependencies.each do |name, constraints|
    spec.add_dependency(name, *constraints)
  end
end
