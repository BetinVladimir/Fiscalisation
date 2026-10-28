const path = require("path");
const { getDefaultConfig } = require("expo/metro-config");

const projectRoot = __dirname;
const workspaceRoot = path.resolve(projectRoot, "..");
const config = getDefaultConfig(projectRoot);

// Keep dependency resolution local to BeeMiniPOS. In particular, do not pick
// up a stale node_modules directory from the former project location.
config.resolver.nodeModulesPaths = [path.resolve(projectRoot, "node_modules")];
config.resolver.disableHierarchicalLookup = true;
// Include nested dependencies but never search ancestor workspaces.
config.resolver.resolveRequest = (context, moduleName, platform) => {
  const singleton = /^(react|react-dom|react-native|react-native-web|react-native-paper|react-native-safe-area-context|react-i18next|i18next)(\/|$)/.test(moduleName);
  const requestedContext = singleton ? {...context, originModulePath: path.join(projectRoot, 'package.json')} : context;
  const localPaths = [];
  let current = path.dirname(requestedContext.originModulePath);
  while (current === projectRoot || current.startsWith(projectRoot + path.sep)) {
    localPaths.push(path.join(current, 'node_modules'));
    current = path.dirname(current);
  }
  return context.resolveRequest({ ...requestedContext, nodeModulesPaths: [...localPaths, ...config.resolver.nodeModulesPaths] }, moduleName, platform);
};

// Authentication artwork is shared by BeeMiniPOS and MiniPOS Web from
// minipos/imgs, which is intentionally outside this Expo package root.
config.watchFolders = [workspaceRoot, path.resolve(projectRoot, '../../../BeeloyBackend/mobile/design-tokens'), path.resolve(projectRoot, '../../../BeeloyBackend/mobile/shared-ui')];
config.resolver.unstable_enableSymlinks = true;

module.exports = config;
