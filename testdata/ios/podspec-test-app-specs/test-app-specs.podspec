Pod::Spec.new do |s|
  s.name = 'FixtureTestAppSpecs'
  s.version = '1.0.0'
  s.summary = 'Test and app specification dependency fixture.'
  s.authors = { 'CocoaPods' => 'info@cocoapods.org' }
  s.license = { :type => 'MIT' }
  s.homepage = 'https://github.com/CocoaPods/CocoaPods'
  s.source = { :git => 'https://github.com/CocoaPods/CocoaPods.git', :tag => s.version.to_s }

  s.test_spec 'UnitTests' do |test_spec|
    test_spec.dependency 'Expecta', '~> 1.0'
    test_spec.dependency 'OCMock', '>= 3.9'
  end

  s.app_spec 'DemoApp' do |app_spec|
    app_spec.dependency 'AFNetworking', '~> 4.0'
  end
end
