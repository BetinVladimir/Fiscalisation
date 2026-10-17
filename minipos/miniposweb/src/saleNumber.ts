/** Plain decimal entry: monetary cents or quantity thousandths, no exponent/rounding. */
export function parseSaleNumber(text: string, decimals: number): number | null {
  const normalized=text.trim().replace(',','.');
  if (!new RegExp(`^(?:0|[1-9]\\d*)(?:\\.\\d{1,${decimals}})?$`).test(normalized)) return null;
  const value=Number(normalized);
  return Number.isFinite(value)&&Number.isSafeInteger(Math.round(value*10**decimals))?value:null;
}
