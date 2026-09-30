const path = require('node:path');
const { getDefaultConfig } = require('expo/metro-config');

const projectRoot = __dirname;
const tokensRoot = path.resolve(projectRoot, '../../BeeloyBackend/mobile/design-tokens');
const config = getDefaultConfig(projectRoot);
config.watchFolders = [...(config.watchFolders || []), tokensRoot];
config.resolver.unstable_enableSymlinks = true;
config.resolver.nodeModulesPaths = [path.resolve(projectRoot, 'node_modules')];
module.exports = config;
