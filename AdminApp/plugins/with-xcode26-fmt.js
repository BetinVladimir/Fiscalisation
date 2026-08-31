const { withPodfile } = require('@expo/config-plugins');

module.exports = function withXcode26Fmt(config) {
  return withPodfile(config, (mod) => {
    const marker = '    # This is necessary for Xcode 14';
    if (!mod.modResults.contents.includes(marker)) {
      throw new Error('Unable to patch Podfile for Xcode 26 fmt compatibility');
    }
    const patch = `    # fmt 11 from React Native 0.79 is not consteval-compatible with Xcode 26.
    fmt_base_header = File.join(installer.sandbox.root, 'fmt/include/fmt/base.h')
    if File.exist?(fmt_base_header)
      File.chmod(0644, fmt_base_header)
      contents = File.read(fmt_base_header)
      contents = contents.sub('#if !defined(__cpp_lib_is_constant_evaluated)', '#if 1')
      File.write(fmt_base_header, contents)
    end

    installer.pods_project.targets.each do |target|
      next unless target.name == 'fmt'

      target.build_configurations.each do |build_config|
        definitions = build_config.build_settings['GCC_PREPROCESSOR_DEFINITIONS'] || ['$(inherited)']
        definitions = [definitions] unless definitions.is_a?(Array)
        definitions << 'FMT_USE_CONSTEVAL=0' unless definitions.include?('FMT_USE_CONSTEVAL=0')
        build_config.build_settings['GCC_PREPROCESSOR_DEFINITIONS'] = definitions
      end
    end

`;
    mod.modResults.contents = mod.modResults.contents.replace(marker, patch + marker);
    return mod;
  });
};
