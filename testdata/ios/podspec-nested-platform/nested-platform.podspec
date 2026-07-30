Pod::Spec.new do |s|
  s.name = 'FixtureNestedPlatform'
  s.version = '1.0.0'
  s.summary = 'Nested platform and subspec dependency fixture.'
  s.authors = { 'CocoaPods' => 'info@cocoapods.org' }
  s.license = { :type => 'MIT' }
  s.homepage = 'https://github.com/CocoaPods/CocoaPods'
  s.source = { :git => 'https://github.com/CocoaPods/CocoaPods.git', :tag => s.version.to_s }

  s.subspec 'Feature' do |feature|
    feature.subspec 'UI' do |ui|
      ui.ios.dependency 'SDWebImage', '~> 5.19'
      ui.osx.dependency 'PromiseKit', '~> 6.18'
    end
  end
end
