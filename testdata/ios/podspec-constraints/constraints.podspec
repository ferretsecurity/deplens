Pod::Spec.new do |s|
  s.name = 'FixtureConstraints'
  s.version = '1.0.0'
  s.summary = 'Podspec dependency constraint fixture.'
  s.authors = { 'CocoaPods' => 'info@cocoapods.org' }
  s.license = { :type => 'MIT' }
  s.homepage = 'https://github.com/CocoaPods/CocoaPods'
  s.source = { :git => 'https://github.com/CocoaPods/CocoaPods.git', :tag => s.version.to_s }

  s.dependency 'Alamofire'
  s.dependency 'AFNetworking', '= 4.0.1'
  s.dependency 'RxSwift', '~> 6.5'
  s.dependency 'Quick', '>= 7.0', '< 8.0'
  s.dependency('Nimble', '~> 12.0')
end
