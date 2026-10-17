import { expect, test } from '@playwright/test';

// Deferred until every app reaches the global Paper + authored-tests gate.
for (const [size, width, height] of [['xs', 360, 800], ['md', 960, 900], ['lg', 1440, 1000]] as const) {
  for (const theme of ['light', 'dark'] as const) {
    test(`Auth rejects malformed challenge and retains editable email ${size}/${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      let attempts = 0;
      await page.route('http://fiscal-admin.test/public/v1/app-auth/challenges', async route => {
        attempts++;
        await route.fulfill({ contentType: 'application/json', body: JSON.stringify(attempts === 1 ? {} : { temporary_token: 'temporary-valid' }) });
      });
      await page.goto('/');
      const email = page.getByTestId('admin-login-email');
      const start = page.getByTestId('admin-login-start');
      await email.fill('invalid');
      await expect(start).toBeDisabled();
      await email.fill('qa@example.org');
      await start.click();
      await expect(page.getByRole('alert')).toContainText('INVALID_CHALLENGE');
      await expect(email).toHaveValue('qa@example.org');
      await expect(email).toBeEditable();
      await start.click();
      await expect(page.getByTestId('admin-login-code')).toBeVisible();
      expect(attempts).toBe(2);
      await page.getByTestId('admin-login-code').fill('12ab34');
      await expect(page.getByTestId('admin-login-code')).toHaveValue('1234');
      await expect(page.getByTestId('admin-login-verify')).toBeDisabled();
    });

    test(`Auth pending request excludes edits and duplicate submit ${size}/${theme}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      let attempts = 0;
      let release!: () => void;
      const responseGate = new Promise<void>(resolve => { release = resolve; });
      await page.route('http://fiscal-admin.test/public/v1/app-auth/challenges', async route => {
        attempts++;
        await responseGate;
        await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ temporary_token: 'temporary-delayed' }) });
      });
      try {
        await page.goto('/');
        const email = page.getByTestId('admin-login-email');
        const start = page.getByTestId('admin-login-start');
        await email.fill('qa@example.org');
        await start.click();
        await expect.poll(() => attempts).toBe(1);
        await expect(start).toBeDisabled();
        await expect(email).not.toBeEditable();
        // Programmatic activation still must not cross the pending gate.
        await start.evaluate(element => (element as HTMLElement).click());
        release();
        await expect(page.getByTestId('admin-login-code')).toBeVisible();
        expect(attempts).toBe(1);
      } finally { release(); }
    });
  }
}
