# ruby project.rb APP_ROOT OUTPUT_DIRECTORY (requires CocoaPods' xcodeproj gem)
require 'xcodeproj'
require 'fileutils'
root=File.expand_path(ARGV.fetch(0)); directory=File.expand_path(ARGV.fetch(1))
FileUtils.mkdir_p(directory)
p=Xcodeproj::Project.new(File.join(directory,'NativeCrypto.xcodeproj'))
t=p.new_target(:application,'NativeCrypto',:ios,'15.1')
['modules/beeloy-execution-crypto/ios/ExecutionKeyStore.swift','tests/native-crypto-ios/Main.swift'].each{|source| t.source_build_phase.add_file_reference(p.main_group.new_file(File.join(root,source)))}
t.build_configurations.each do |c|
 c.build_settings['PRODUCT_BUNDLE_IDENTIFIER']='com.beeloy.native-provider-test'
 c.build_settings['SWIFT_VERSION']='5.0'
 c.build_settings['GENERATE_INFOPLIST_FILE']='YES'
 c.build_settings['INFOPLIST_KEY_UILaunchScreen_Generation']='YES'
 c.build_settings['CODE_SIGN_STYLE']='Automatic'
 c.build_settings['TARGETED_DEVICE_FAMILY']='1,2'
 c.build_settings['SDKROOT']='iphoneos'
end
p.save
