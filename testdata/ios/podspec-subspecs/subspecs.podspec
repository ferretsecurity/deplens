Pod::Spec.new do |s|
  s.name = 'FixtureSubspecs'
  s.version = '1.0.0'
  s.summary = 'Subspec podspec dependency fixture.'
  s.authors = { 'CocoaPods' => 'info@cocoapods.org' }
  s.license = { :type => 'MIT' }
  s.homepage = 'https://github.com/CocoaPods/CocoaPods'
  s.source = { :git => 'https://github.com/CocoaPods/CocoaPods.git', :tag => s.version.to_s }

  s.subspec 'Core' do |core|
    core.dependency 'FixtureSubspecs/Support'
    core.dependency 'Alamofire/NetworkActivityIndicator', '~> 5.9'
  end

  s.subspec 'Support' do |support|
    support.dependency 'Reachability', '~> 3.2'
  end
end
