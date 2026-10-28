// Working-screen preference regression. Synthetic authority fixture; not ordinary authentication or live fiscal acceptance.
import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { primaryAuthorityFixture } from "./primary-authority-fixture";
for (const [width, theme] of [
  [320, "light"],
  [320, "dark"],
  [840, "light"],
  [840, "dark"],
  [1440, "light"],
  [1440, "dark"],
] as const)
  test(`working preferences ${width}/${theme}`, async ({ page }, info) => {
    const f = await primaryAuthorityFixture(page),
      scans: any[] = [];
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.setViewportSize({ width, height: 900 });
    await page.addInitScript(
      ({ theme }) => {
        localStorage.setItem("minipos-language", "en");
        localStorage.setItem("minipos-web-theme", theme);
      },
      { theme },
    );
    const settle = async (kind: string) => {
      await page.waitForFunction(
        async (prefix) => {
          const el = document.querySelector<HTMLElement>(
            `[data-testid="${prefix}-surface"]`,
          );
          if (!el) return false;
          let stable = 0;
          for (let i = 0; i < 60; i++) {
            await new Promise(requestAnimationFrame);
            if (!el.isConnected) return false;
            const c = getComputedStyle(el),
              p = el.parentElement && getComputedStyle(el.parentElement);
            if (
              c.opacity === "1" &&
              c.transform === "matrix(1, 0, 0, 1, 0, 0)" &&
              p?.transform === "matrix(1, 0, 0, 1, 0, 0)"
            ) {
              if (++stable === 6) return true;
            } else stable = 0;
          }
          return false;
        },
        "app-" + kind + "-menu",
      );
    };
    const scan = async (state: string) => {
      await page.evaluate(async () => {
        await document.fonts.ready;
      });
      await expect(page.locator("[data-paper-ripple]")).toHaveCount(0);
      const raw = await new AxeBuilder({ page })
        .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
        .analyze();
      scans.push({ state, raw });
      await info.attach(state, {
        body: await page.screenshot({ fullPage: false }),
        contentType: "image/png",
      });
      expect(raw.violations).toEqual([]);
      for (const issue of raw.incomplete) {
        expect(issue.id).toBe("color-contrast");
        for (const node of issue.nodes) {
          const selector = node.target.join(" ");
          expect(selector).toMatch(/app-language-/);
          await page
            .locator(selector)
            .evaluate((el) =>
              el.scrollIntoView({ block: "center", inline: "nearest" }),
            );
          const textRect = await page.locator(selector).boundingBox(),
            scrollRect = await page
              .getByTestId("app-language-scroll")
              .boundingBox();
          expect(textRect!.x).toBeGreaterThanOrEqual(
            Math.max(0, scrollRect!.x) - 1,
          );
          expect(textRect!.x + textRect!.width).toBeLessThanOrEqual(
            Math.min(width, scrollRect!.x + scrollRect!.width) + 1,
          );
          expect(textRect!.y).toBeGreaterThanOrEqual(scrollRect!.y - 1);
          expect(textRect!.y + textRect!.height).toBeLessThanOrEqual(
            scrollRect!.y + scrollRect!.height + 1,
          );
          const focused = await new AxeBuilder({ page })
            .include(selector)
            .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
            .analyze();
          scans.push({
            state: state + "-visible-text:" + selector,
            raw: focused,
          });
          expect([...focused.violations, ...focused.incomplete]).toEqual([]);
        }
      }
    };
    try {
      await page.goto("/");
      await expect(page.getByTestId("sale-products")).toBeVisible();
      await page.waitForLoadState("networkidle");
      await page.getByTestId("app-language-open").click();
      await expect(page.getByTestId("app-language-scroll")).toBeVisible();
      await settle("language");
      await expect(page.getByTestId("app-language-en")).toBeFocused();
      const initial = await page.getByTestId("app-language-en").boundingBox(),
        clip = await page.getByTestId("app-language-scroll").boundingBox();
      expect(initial!.y).toBeGreaterThanOrEqual(clip!.y - 1);
      expect(initial!.y + initial!.height).toBeLessThanOrEqual(
        clip!.y + clip!.height + 1,
      );
      await scan("language-open");
      const b = await page.getByTestId("app-language-scroll").boundingBox();
      expect(b!.x).toBeGreaterThanOrEqual(0);
      expect(b!.x + b!.width).toBeLessThanOrEqual(width);
      await page.keyboard.press("End");
      await expect(page.getByTestId("app-language-ru")).toBeFocused();
      const metrics = await page
        .getByTestId("app-language-scroll")
        .evaluate((el) => ({
          top: el.scrollTop,
          client: el.clientHeight,
          total: el.scrollHeight,
        }));
      expect(metrics.total).toBeGreaterThan(metrics.client);
      expect(metrics.top).toBeGreaterThan(0);
      await info.attach("last-language-visible", {
        body: await page.screenshot({ fullPage: false }),
        contentType: "image/png",
      });
      await page.keyboard.press("Escape");
      await expect(page.getByTestId("app-language-open")).toBeFocused();
      await page.getByTestId("app-theme-open").click();
      await settle("theme");
      await scan("theme-open");
      await page.keyboard.press("Escape");
      await expect(page.getByTestId("app-theme-open")).toBeFocused();
      f.verifyNoSale();
    } finally {
      await info.attach("raw-evidence", {
        body: JSON.stringify({ scans, errors: f.errors, ledger: f.ledger }),
        contentType: "application/json",
      });
    }
  });
