import type { Page } from '@playwright/test';

export async function doubleText(page: Page) {
  await page.evaluate(() => {
    const seen = new WeakSet<Element>();
    const scale = () => {
      const targets = Array.from(document.querySelectorAll<HTMLElement>('[dir="auto"]')).filter(node => !seen.has(node));
      const metrics = targets.map(node => { const style = getComputedStyle(node); return { node, size: parseFloat(style.fontSize), line: parseFloat(style.lineHeight) }; });
      for (const { node, size, line } of metrics) {
        seen.add(node);
        node.dataset.originalFontSize = String(size);
        node.style.fontSize = `${size * 2}px`;
        if (Number.isFinite(line)) node.style.lineHeight = `${line * 2}px`;
      }
    };
    scale();
    new MutationObserver(scale).observe(document.body, { childList: true, subtree: true });
  });
}
