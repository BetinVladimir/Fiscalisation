const path = require('node:path');
const { getDefaultConfig } = require('expo/metro-config');

const projectRoot = __dirname;
const tokensRoot = path.resolve(projectRoot, '../../BeeloyBackend/mobile/design-tokens');
const config = getDefaultConfig(projectRoot);
config.watchFolders = [...(config.watchFolders || []), tokensRoot, path.resolve(projectRoot, '../../BeeloyBackend/mobile/shared-ui')];
config.resolver.unstable_enableSymlinks = true;
config.resolver.nodeModulesPaths = [path.resolve(projectRoot, 'node_modules')];
config.resolver.resolveRequest = (context, moduleName, platform) => {
  const singleton = /^(react|react-dom|react-native|react-native-web|react-native-paper|react-native-safe-area-context|react-i18next|i18next)(\/|$)/.test(moduleName);
  return context.resolveRequest(singleton ? {...context, originModulePath: path.join(projectRoot, 'package.json')} : context, moduleName, platform);
};
module.exports = config;
