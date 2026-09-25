const manifest = require('./app.json');
module.exports = function applyStoreConfig(input) {
  const config = { ...(input.expo || {}), ...input };
  delete config.expo;
  delete config.newArchitecture;
  const variant = process.env.APP_VARIANT || 'production';
  if (!['production', 'demo'].includes(variant)) throw new Error('APP_VARIANT must be production or demo');
  const suffix = variant === 'demo' ? 'demo' : 'prod';
  const id = `org.beeloy.${manifest.app}.${suffix}`;
  const domain = `${suffix}.${manifest.app}.beeloy.org`;
  const projectId = manifest.projectId || config.extra?.eas?.projectId;
  if (process.env.BEELOY_STORE_BUILD === '1') {
    for (const [key, value] of Object.entries(process.env)) {
      if (key.startsWith('EXPO_PUBLIC_E2E_') && value && value !== 'false') throw new Error(`${key} must not be enabled in store builds`);
    }
  }
  const plugins = (config.plugins || []).filter(p => !String(Array.isArray(p) ? p[0] : p).includes('with-xcode26-fmt'));
  if (!plugins.some(p => (Array.isArray(p) ? p[0] : p) === 'expo-build-properties')) {
    plugins.push(['expo-build-properties', { android: { compileSdkVersion: 36, targetSdkVersion: 36 }, ios: { deploymentTarget: '15.1' } }]);
  }
  return {
    ...config,
    name: manifest.name + (suffix === 'demo' ? ' Demo' : ''),
    version: manifest.version,
    owner: manifest.owner || config.owner,
    icon: './store/icon.png',
    newArchEnabled: true,
    scheme: `beeloy-${manifest.app}-${suffix}`,
    ios: { ...config.ios, bundleIdentifier: id, buildNumber: config.ios?.buildNumber || '1', ...(config.ios?.entitlements?.['keychain-access-groups'] ? { entitlements: { ...config.ios.entitlements, 'keychain-access-groups': ['$(AppIdentifierPrefix)' + id] } } : {}), icon: './store/icon.png' },
    android: { ...config.android, package: id, versionCode: config.android?.versionCode || 1, icon: './store/icon.png', adaptiveIcon: { foregroundImage: './store/adaptive-icon.png', backgroundColor: '#11252E' } },
    web: { ...config.web, favicon: './store/graphics/thumbnail-128.png' },
    extra: { ...config.extra, appVariant: variant, isDemo: suffix === 'demo', webOrigin: `https://${domain}`, eas: { ...config.extra?.eas, ...(projectId ? { projectId } : {}) } },
    plugins,
  };
};
