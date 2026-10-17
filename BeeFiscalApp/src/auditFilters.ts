/** RFC3339 bounds used by the audit API; empty bounds are optional. */
function timestamp(value: string) {
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(Z|[+-]\d{2}:\d{2})$/.exec(value);
  if (!match) return NaN;
  const [, year, month, day, hour, minute, second, zone] = match;
  const y = Number(year), m = Number(month), d = Number(day);
  const days = [31, y % 4 === 0 && (y % 100 !== 0 || y % 400 === 0) ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
  if (m < 1 || m > 12 || d < 1 || d > days[m - 1] || Number(hour) > 23 || Number(minute) > 59 || Number(second) > 59) return NaN;
  if (zone !== 'Z' && (Number(zone.slice(1, 3)) > 23 || Number(zone.slice(4)) > 59)) return NaN;
  return Date.parse(value);
}
export function auditFilterDatesValid(filters: { from: string; to: string }) {
  const from = filters.from.trim(), to = filters.to.trim();
  const start = from ? timestamp(from) : -Infinity, end = to ? timestamp(to) : Infinity;
  return !Number.isNaN(start) && !Number.isNaN(end) && start <= end;
}
