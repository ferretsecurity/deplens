Pod::Spec.new do |s|
  s.name = 'FixtureConfigurations'
  s.version = '1.0.0'
  s.summary = 'Configuration-scoped dependency fixture.'
  s.authors = { 'CocoaPods' => 'info@cocoapods.org' }
  s.license = { :type => 'MIT' }
  s.homepage = 'https://github.com/CocoaPods/CocoaPods'
  s.source = { :git => 'https://github.com/CocoaPods/CocoaPods.git', :tag => s.version.to_s }

  s.dependency 'FLEX', '~> 5.22', :configurations => ['Debug']
  s.dependency 'CocoaLumberjack/Swift', '>= 3.8', :configurations => :debug
end
