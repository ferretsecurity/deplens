Pod::Spec.new do |s|
  s.name = 'FixturePlatforms'
  s.version = '1.0.0'
  s.summary = 'Platform-scoped podspec dependency fixture.'
  s.authors = { 'CocoaPods' => 'info@cocoapods.org' }
  s.license = { :type => 'MIT' }
  s.homepage = 'https://github.com/CocoaPods/CocoaPods'
  s.source = { :git => 'https://github.com/CocoaPods/CocoaPods.git', :tag => s.version.to_s }

  s.ios.dependency 'MBProgressHUD', '~> 1.2'
  s.osx.dependency 'Sparkle', '~> 2.5'
  s.macos.dependency 'Sparkle', '~> 2.5'
  s.tvos.dependency 'TVVLCKit', '~> 3.5'
  s.watchos.dependency 'KeychainAccess', '~> 4.2'
  s.visionos.dependency 'Kingfisher', '~> 7.0'
end
