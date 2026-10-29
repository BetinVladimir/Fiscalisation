// AUTH-001 regression: usePlatformOidc must not call useAutoDiscovery with an
// empty string when OIDC env vars are missing — that throws "Expected a valid
// discovery object or issuer URL" before the user interacts with the page.
//
// This test inspects the module source to assert that null (not "") is passed
// as the fallback argument, and verifies the configured flag logic.

import * as fs from 'fs';
import * as path from 'path';

const source = fs.readFileSync(path.join(__dirname, 'usePlatformOidc.ts'), 'utf8');

describe('usePlatformOidc — AUTH-001 discovery guard', () => {
  it('passes null (not empty string) to useAutoDiscovery when not configured', () => {
    // The fix: `configured ? issuer : null` must appear — NOT `configured ? issuer : ""`
    expect(source).toContain('configured ? issuer : null');
    expect(source).not.toMatch(/configured\s*\?\s*issuer\s*:\s*""/);
  });

  it('configured flag requires https issuer AND non-empty clientId', () => {
    // The configured flag must require both conditions together
    expect(source).toMatch(/issuer\.startsWith\("https:\/\/"\)\s*&&\s*!!clientId/);
  });

  it('useAuthRequest second argument is guarded by configured', () => {
    // Request must only be created with real discovery (not null discovery object)
    expect(source).toContain('configured ? discovery : null');
  });

  it('ready requires both configured and discovery and request', () => {
    expect(source).toMatch(/ready:\s*configured\s*&&\s*!!request\s*&&\s*!!discovery/);
  });
});
