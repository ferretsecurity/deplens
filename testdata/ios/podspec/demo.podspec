Pod::Spec.new do |s|
  s.name         = "Demo"
  s.version      = "1.0.0"
  s.summary      = "Complete Podspec dependency fixture."
  s.authors      = { "CocoaPods" => "info@cocoapods.org" }
  s.license      = { :type => "MIT" }
  s.homepage     = "https://github.com/CocoaPods/CocoaPods"
  s.source       = { :git => "https://github.com/CocoaPods/CocoaPods.git", :tag => s.version.to_s }
  s.dependency "Alamofire", "~> 5.0"
end
