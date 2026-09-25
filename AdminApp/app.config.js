const base = require('./app.json').expo;
module.exports = () => { const isDemo = process.env.APP_VARIANT === 'demo'; const id = isDemo ? 'org.beeloy.fiscal.admin.demo' : 'org.beeloy.fiscal.admin'; const origin = isDemo ? 'https://demo-api.beeloy.org' : 'https://api.beeloy.org'; return { ...base, runtimeVersion: { policy: 'appVersion' }, name: isDemo ? 'BeeFiscal Admin (Demo)' : base.name, scheme: isDemo ? 'beefiscalplatformadmin-demo' : base.scheme, ios: { ...base.ios, bundleIdentifier: id }, android: { ...base.android, package: id }, extra: { ...base.extra, appVariant: isDemo ? 'demo' : 'production', isDemo, apiBaseUrl: origin }, plugins: [...(base.plugins ?? []), './plugins/with-xcode26-fmt'] }; };

// Canonical prod/demo store identities. Keep app-specific configuration above.
const originalConfig = module.exports;
module.exports = (context) => require('./store/config.cjs')(typeof originalConfig === 'function' ? originalConfig(context) : originalConfig);
