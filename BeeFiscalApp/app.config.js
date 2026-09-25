const base = require('./app.json').expo;
module.exports = () => { const isDemo = process.env.APP_VARIANT === 'demo'; const id = isDemo ? 'com.beefiscal.app.demo' : 'com.beefiscal.app'; const origin = isDemo ? 'https://demo-api.beeloy.org' : 'https://api.beeloy.org'; return { ...base, runtimeVersion: { policy: 'appVersion' }, name: isDemo ? 'BeeFiscal (Demo)' : base.name, scheme: isDemo ? 'beefiscalapp-demo' : base.scheme, ios: { ...base.ios, bundleIdentifier: id }, android: { ...base.android, package: id }, extra: { ...base.extra, appVariant: isDemo ? 'demo' : 'production', isDemo, apiBaseUrl: `${origin}/public/v1` }, plugins: [...(base.plugins ?? []), './plugins/with-xcode26-fmt'] }; };

// Canonical prod/demo store identities. Keep app-specific configuration above.
const originalConfig = module.exports;
module.exports = (context) => require('./store/config.cjs')(typeof originalConfig === 'function' ? originalConfig(context) : originalConfig);
