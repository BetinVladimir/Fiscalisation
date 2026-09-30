import { expect, test } from '@playwright/test';
import sharp from 'sharp';
import { collectPixelEvidence } from './contrast-evidence';

const width = 80;
const height = 40;
const candidate = {
  target: '#sample',
  foreground: 'rgb(0, 0, 0)',
  background: 'rgb(255, 255, 255)',
  effectiveOpacity: 1,
  rect: { x: 10, y: 5, width: 40, height: 25 },
  viewport: { x: 0, y: 0, width, height },
};

async function image(pixel: (x: number, y: number) => [number, number, number]) {
  const data = Buffer.alloc(width * height * 4);
  for (let y = 0; y < height; y++) for (let x = 0; x < width; x++) {
    const offset = (y * width + x) * 4;
    const [red, green, blue] = pixel(x, y);
    data[offset] = red;
    data[offset + 1] = green;
    data[offset + 2] = blue;
    data[offset + 3] = 255;
  }
  return sharp(data, { raw: { width, height, channels: 4 } }).png().toBuffer();
}

test('pixel evidence supports visible high-contrast foreground on a flat background', async () => {
  const png = await image((x, y) => x >= 20 && x < 30 && y >= 10 && y < 20 ? [0, 0, 0] : [255, 255, 255]);
  const [result] = await collectPixelEvidence(png, [candidate]);
  expect(result.pixelEvidence.status).toBe('candidate-supported');
  expect(result.pixelEvidence.pixelRatio).toBe(21);
});

test('pixel evidence refuses invisible, low-contrast and patterned cases', async () => {
  const invisible = await image(() => [255, 255, 255]);
  const lowContrast = await image((x, y) => x >= 20 && x < 30 && y >= 10 && y < 20 ? [170, 170, 170] : [255, 255, 255]);
  const patterned = await image((x, y) => x >= 7 && x < 53 && y >= 2 && y < 33 && x % 2 === 0 ? [0, 0, 0] : [255, 255, 255]);
  const borderOnly = await image((x, y) => x >= 10 && x < 50 && y >= 5 && y < 30 && (x === 10 || x === 49 || y === 5 || y === 29) ? [0, 0, 0] : [255, 255, 255]);
  const [a] = await collectPixelEvidence(invisible, [candidate]);
  const [b] = await collectPixelEvidence(lowContrast, [{ ...candidate, foreground: 'rgb(170, 170, 170)' }]);
  const [c] = await collectPixelEvidence(patterned, [candidate]);
  const [d] = await collectPixelEvidence(borderOnly, [candidate]);
  expect(a.pixelEvidence.status).toBe('manual-review');
  expect(b.pixelEvidence.status).toBe('manual-review');
  expect(c.pixelEvidence.status).toBe('manual-review');
  expect(d.pixelEvidence.status).toBe('manual-review');
});
