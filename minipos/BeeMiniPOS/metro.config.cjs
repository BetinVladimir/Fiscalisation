const path = require("path");
const { getDefaultConfig } = require("expo/metro-config");

const projectRoot = __dirname;
const workspaceRoot = path.resolve(projectRoot, "..");
const config = getDefaultConfig(projectRoot);

// Keep dependency resolution local to BeeMiniPOS. In particular, do not pick
// up a stale node_modules directory from the former project location.
config.resolver.nodeModulesPaths = [path.resolve(projectRoot, "node_modules")];
config.resolver.disableHierarchicalLookup = true;

// Authentication artwork is shared by BeeMiniPOS and MiniPOS Web from
// minipos/imgs, which is intentionally outside this Expo package root.
config.watchFolders = [workspaceRoot];

module.exports = config;
