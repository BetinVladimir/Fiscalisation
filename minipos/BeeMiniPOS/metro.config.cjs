const path = require("path");
const { getDefaultConfig } = require("expo/metro-config");

const projectRoot = __dirname;
const workspaceRoot = path.resolve(projectRoot, "..");
const config = getDefaultConfig(projectRoot);

// Authentication artwork is shared by BeeMiniPOS and MiniPOS Web from
// minipos/imgs, which is intentionally outside this Expo package root.
config.watchFolders = [workspaceRoot];

module.exports = config;
