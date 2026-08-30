Pod::Spec.new do |spec|
  spec.name = "SourceVersion"
  spec.source = { :git => "https://example.test/source-version.git", :tag => spec.version }
end
