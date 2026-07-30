Gem::Specification.new do |s|
  s.name = "call-styles"
  s.version = "1.0.0"
  s.summary = "Parenthesized and array requirement calls"
  s.authors = ["Fixture Author"]
  s.files = []

  s.add_dependency("parenthesized-kit", ">= 1.0")
  s.add_dependency("array-kit", [">= 2.2.0", "< 3.0"])
  s.add_development_dependency("array-dev-kit", ["~> 2.4"])
end
