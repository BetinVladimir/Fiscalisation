import { expect, type Locator } from '@playwright/test';

type Color = [number, number, number, number];
function parse(value: string): Color {
  const match = value.match(/^rgba?\(\s*([\d.]+)[, ]+([\d.]+)[, ]+([\d.]+)(?:\s*[,/]\s*([\d.]+))?\s*\)$/);
  if (!match) throw new Error(`Unsupported computed color: ${value}`);
  return [Number(match[1]), Number(match[2]), Number(match[3]), match[4] === undefined ? 1 : Number(match[4])];
}
function over(top: Color, bottom: Color): Color {
  return [0, 1, 2].map(i => top[i] * top[3] + bottom[i] * (1 - top[3])).concat(1) as Color;
}
function ratio(a: Color, b: Color) {
  const luminance = (c: Color) => c.slice(0, 3).map(v => v / 255).map(v => v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4).reduce((sum, v, i) => sum + v * [.2126, .7152, .0722][i], 0);
  const x = luminance(a), y = luminance(b);
  return (Math.max(x, y) + .05) / (Math.min(x, y) + .05);
}
export async function frameContrast(action: Locator, colorProperty: 'borderTopColor' | 'outlineColor' = 'borderTopColor') {
  const layers = await action.evaluate((element, colorProperty) => {
    const chain: { background: string; opacity: string; image: string; filter: string; blend: string }[] = [];
    for (let node: Element | null = element; node; node = node.parentElement) {
      const s = getComputedStyle(node);
      chain.push({ background: s.backgroundColor, opacity: s.opacity, image: s.backgroundImage, filter: s.filter, blend: s.mixBlendMode });
    }
    const style = getComputedStyle(element);
    return { chain: chain.reverse(), border: style[colorProperty] };
  }, colorProperty);
  let outside: Color = [255, 255, 255, 1];
  for (const layer of layers.chain.slice(0, -1)) {
    expect(Number(layer.opacity), 'Ancestor opacity must be supported').toBe(1);
    expect(layer.image).toBe('none'); expect(layer.filter).toBe('none'); expect(layer.blend).toBe('normal');
    outside = over(parse(layer.background), outside);
  }
  const own = layers.chain.at(-1)!;
  expect(Number(own.opacity)).toBe(1); expect(own.image).toBe('none'); expect(own.filter).toBe('none'); expect(own.blend).toBe('normal');
  const inside = over(parse(own.background), outside);
  const border = over(parse(layers.border), inside);
  return { insideRatio: ratio(border, inside), outsideRatio: ratio(border, outside), border: layers.border, inside, outside };
}
