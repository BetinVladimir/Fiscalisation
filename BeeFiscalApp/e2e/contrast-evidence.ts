import type { Page, TestInfo } from '@playwright/test';
import type AxeBuilder from '@axe-core/playwright';
import { writeFile } from 'node:fs/promises';
import sharp from 'sharp';

type AxeResults = Awaited<ReturnType<AxeBuilder['analyze']>>;
type ContrastCandidate = {
  target: string;
  text?: string;
  error?: string;
  foreground?: string;
  background?: string | null;
  effectiveOpacity?: number;
  rect?: { x: number; y: number; width: number; height: number };
  viewport?: { x: number; y: number; width: number; height: number };
  contrastCandidate?: number | null;
};

function parseRgb(value: string): number[] | null {
  const match = /^rgb\((\d+),\s*(\d+),\s*(\d+)\)$/.exec(value);
  return match ? match.slice(1).map(Number) : null;
}

function contrastRatio(foreground: string, background: string): number | null {
  const front = parseRgb(foreground);
  const back = parseRgb(background);
  if (!front || !back) return null;
  const luminance = (rgb: number[]) => rgb.map(value => {
    const channel = value / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  }).reduce((sum, value, index) => sum + value * [0.2126, 0.7152, 0.0722][index], 0);
  const values = [luminance(front), luminance(back)].sort((a, b) => b - a);
  return Number(((values[0] + 0.05) / (values[1] + 0.05)).toFixed(2));
}

export async function collectContrastCandidates(page: Page, results: AxeResults): Promise<ContrastCandidate[]> {
  const candidates: ContrastCandidate[] = [];
  for (const node of results.incomplete.filter(item => item.id === 'color-contrast').flatMap(item => item.nodes)) {
    const target = String(node.target[0]);
    const candidate = await page.evaluate(selector => {
      const element = document.querySelector(selector);
      if (!element) return { target: selector, error: 'Axe target missing after scan' };
      const style = getComputedStyle(element);
      const layers: string[] = [];
      let effectiveOpacity = 1;
      for (let current: Element | null = element; current; current = current.parentElement) {
        const currentStyle = getComputedStyle(current);
        effectiveOpacity *= Number(currentStyle.opacity);
        const color = currentStyle.backgroundColor;
        if (color && color !== 'transparent' && color !== 'rgba(0, 0, 0, 0)') layers.unshift(color);
      }
      const box = element.getBoundingClientRect();
      const parse = (color: string) => {
        const match = /^rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?\)$/.exec(color);
        return match ? [Number(match[1]), Number(match[2]), Number(match[3]), match[4] === undefined ? 1 : Number(match[4])] : null;
      };
      let composed = [255, 255, 255];
      let known = false;
      for (const layer of layers) {
        const rgba = parse(layer);
        if (!rgba) { known = false; break; }
        composed = rgba.slice(0, 3).map((value, index) => value * rgba[3] + composed[index] * (1 - rgba[3]));
        if (rgba[3] === 1) known = true;
      }
      const background = known ? `rgb(${composed.map(value => Math.round(value)).join(', ')})` : null;
      return {
        target: selector,
        text: element.textContent?.trim().slice(0, 120) ?? '',
        foreground: style.color,
        fontSize: style.fontSize,
        fontWeight: style.fontWeight,
        opacity: style.opacity,
        effectiveOpacity,
        rect: { x: box.left + scrollX, y: box.top + scrollY, width: box.width, height: box.height },
        viewport: { x: scrollX, y: scrollY, width: innerWidth, height: innerHeight },
        backgroundLayers: layers,
        background,
        note: 'Candidate only: Axe detected overlapping layers; inspect the screenshot before accepting a ratio.',
      };
    }, target);
    candidates.push({ ...candidate, contrastCandidate: 'foreground' in candidate && candidate.background ? contrastRatio(candidate.foreground, candidate.background) : null });
  }
  return candidates;
}

export async function collectPixelEvidence(png: Buffer, candidates: ContrastCandidate[]) {
  const { data, info } = await sharp(png).ensureAlpha().raw().toBuffer({ resolveWithObject: true });
  return candidates.map(candidate => {
    if (!candidate.rect || !candidate.viewport || !candidate.foreground) return { ...candidate, pixelEvidence: { status: 'missing-geometry' } };
    const { rect, viewport } = candidate;
    const scale = info.width / viewport.width;
    const padding = Math.min(3, Math.floor(Math.min(rect.width, rect.height) / 4));
    const left = Math.max(rect.x - padding, viewport.x);
    const top = Math.max(rect.y - padding, viewport.y);
    const right = Math.min(rect.x + rect.width + padding, viewport.x + viewport.width);
    const bottom = Math.min(rect.y + rect.height + padding, viewport.y + viewport.height);
    const x0 = Math.max(0, Math.ceil(left * scale));
    const y0 = Math.max(0, Math.ceil(top * scale));
    const x1 = Math.min(info.width, Math.floor(right * scale));
    const y1 = Math.min(info.height, Math.floor(bottom * scale));
    if (x1 <= x0 || y1 <= y0) return { ...candidate, pixelEvidence: { status: 'offscreen' } };
    const counts = new Map<string, number>();
    let sampled = 0;
    let foregroundPixels = 0;
    const front = parseRgb(candidate.foreground);
    const foregroundInset = Math.min(4, Math.floor(Math.min(rect.width, rect.height) / 6));
    const foregroundLeft = (rect.x + foregroundInset) * scale;
    const foregroundTop = (rect.y + foregroundInset) * scale;
    const foregroundRight = (rect.x + rect.width - foregroundInset) * scale;
    const foregroundBottom = (rect.y + rect.height - foregroundInset) * scale;
    const step = (x1 - x0) * (y1 - y0) < 10000 ? 1 : 2;
    for (let y = y0; y < y1; y += step) for (let x = x0; x < x1; x += step) {
      const offset = (y * info.width + x) * info.channels;
      if (data[offset + 3] < 255) continue;
      const color = `rgb(${data[offset]}, ${data[offset + 1]}, ${data[offset + 2]})`;
      counts.set(color, (counts.get(color) ?? 0) + 1);
      if (front && x >= foregroundLeft && x < foregroundRight && y >= foregroundTop && y < foregroundBottom && Math.max(...front.map((channel, index) => Math.abs(channel - data[offset + index]))) <= 12) foregroundPixels += 1;
      sampled += 1;
    }
    if (!sampled) return { ...candidate, pixelEvidence: { status: 'no-opaque-pixels' } };
    const [visibleBackground, dominantCount] = [...counts].sort((a, b) => b[1] - a[1])[0];
    const pixelRatio = contrastRatio(candidate.foreground, visibleBackground);
    const share = Number((dominantCount / sampled).toFixed(3));
    const matchesDomBackground = candidate.background === visibleBackground;
    return {
      ...candidate,
      pixelEvidence: {
        status: share >= 0.7 && matchesDomBackground && candidate.effectiveOpacity === 1 && foregroundPixels >= 2 && pixelRatio !== null && pixelRatio >= 4.5 ? 'candidate-supported' : 'manual-review',
        visibleBackground,
        pixelRatio,
        dominantShare: share,
        matchesDomBackground,
        sampledPixels: sampled,
        foregroundPixels,
        note: 'Modal pixel color inside the Axe target is supporting evidence; inspect complex layers and text rendering before accepting WCAG AA.',
      },
    };
  });
}

export async function saveWcagEvidence(page: Page, testInfo: TestInfo, results: AxeResults, details: Record<string, unknown>) {
  const screenshot = await page.screenshot({ path: testInfo.outputPath('screen.png'), fullPage: true });
  const contrastCandidates = await collectContrastCandidates(page, results);
  const pixelCandidates = await collectPixelEvidence(screenshot, contrastCandidates);
  const pixelSummary = {
    candidates: pixelCandidates.length,
    supported: pixelCandidates.filter(candidate => candidate.pixelEvidence.status === 'candidate-supported').length,
    manualReview: pixelCandidates.filter(candidate => candidate.pixelEvidence.status !== 'candidate-supported').length,
  };
  await writeFile(testInfo.outputPath('axe.json'), JSON.stringify({ ...details, violations: results.violations, incomplete: results.incomplete, pixelSummary, contrastCandidates: pixelCandidates }, null, 2));
  return pixelCandidates;
}
