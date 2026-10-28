Pod::Spec.new do |s|
  s.name = 'BeeloyExecutionCrypto'
  s.version = '1.0.0'
  s.summary = 'Non-exportable native execution intent signing keys'
  s.description = s.summary
  s.license = { :type => 'MIT' }
  s.author = 'Beeloy'
  s.homepage = 'https://beeloy.com'
  s.platforms = { :ios => '15.1' }
  s.swift_version = '5.9'
  s.source = { :git => '' }
  s.static_framework = true
  s.dependency 'ExpoModulesCore'
  s.frameworks = 'Security'
  s.source_files = '**/*.{h,m,mm,swift}'
end
