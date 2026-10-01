import type { Page } from '@playwright/test';
import type AxeBuilder from '@axe-core/playwright';

type AxeResults = Awaited<ReturnType<AxeBuilder['analyze']>>;

/** Supporting evidence only. Overlap, gradients and pseudo-elements still need visual review. */
export async function collectContrastCandidates(page: Page, results: AxeResults, screenshot: Buffer) {
  const targets = results.incomplete
    .filter(item => item.id === 'color-contrast')
    .flatMap(item => item.nodes.map(node => String(node.target[0])));
  if (!targets.length) return [];
  return page.evaluate(async ({ targets, png }) => {
    const image = new Image();
    image.src = `data:image/png;base64,${png}`;
    await image.decode();
    const canvas = document.createElement('canvas');
    canvas.width = image.width;
    canvas.height = image.height;
    const context = canvas.getContext('2d', { willReadFrequently: true });
    if (!context) throw new Error('Canvas unavailable for contrast evidence');
    context.drawImage(image, 0, 0);
    const pixel = context.getImageData(0, 0, image.width, image.height).data;
    const luminance = (channels: number[]) => channels.map(value => {
      const channel = value / 255;
      return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
    }).reduce((sum, value, index) => sum + value * [0.2126, 0.7152, 0.0722][index], 0);
    const parse = (color: string) => {
      const match = /^rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?\)$/.exec(color);
      return match ? match.slice(1, 4).map(Number) : null;
    };
    return targets.map(target => {
      let element: Element | null;
      try { element = document.querySelector(target); }
      catch { return { target, status: 'invalid-selector' }; }
      if (!element) return { target, status: 'missing-element' };
      const box = element.getBoundingClientRect();
      const style = getComputedStyle(element);
      const foreground = parse(style.color);
      const scale = image.width / innerWidth;
      const x0 = Math.max(0, Math.floor((box.left + scrollX) * scale));
      const x1 = Math.min(image.width, Math.ceil((box.right + scrollX) * scale));
      const y0 = Math.max(0, Math.floor((box.top + scrollY) * scale));
      const y1 = Math.min(image.height, Math.ceil((box.bottom + scrollY) * scale));
      if (!foreground || x1 <= x0 || y1 <= y0) return { target, status: 'no-visible-geometry' };
      const counts = new Map<string, number>();
      let sampled = 0;
      for (let y = y0; y < y1; y += 2) for (let x = x0; x < x1; x += 2) {
        const offset = (y * image.width + x) * 4;
        if (pixel[offset + 3] !== 255) continue;
        const key = `${pixel[offset]},${pixel[offset + 1]},${pixel[offset + 2]}`;
        counts.set(key, (counts.get(key) || 0) + 1);
        sampled++;
      }
      const [backgroundKey, backgroundCount] = [...counts].sort((a, b) => b[1] - a[1])[0] || [];
      if (!backgroundKey) return { target, status: 'no-opaque-pixels' };
      const background = backgroundKey.split(',').map(Number);
      const light = luminance(foreground), dark = luminance(background);
      const ratio = (Math.max(light, dark) + 0.05) / (Math.min(light, dark) + 0.05);
      const dominantShare = backgroundCount / sampled;
      return {
        target, status: 'manual-review', text: element.textContent?.trim().slice(0, 100) || '',
        foreground: style.color, pixelBackground: `rgb(${background.join(', ')})`,
        contrastCandidate: Number(ratio.toFixed(2)), dominantShare: Number(dominantShare.toFixed(3)),
        fontSize: style.fontSize, fontWeight: style.fontWeight,
        note: 'Screenshot modal color within target bounds; not a WCAG verdict. Check actual glyph background, opacity, overlays and pseudo-elements.',
      };
    });
  }, { targets, png: screenshot.toString('base64') });
}
