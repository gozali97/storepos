export function summarizeSales(rows) {
  const sum = key => rows.some(row => row[key] == null) ? null : rows.reduce((total, row) => total + BigInt(row[key]), 0n);
  return { net: sum('net_cents'), discount: sum('discount_cents'), returns: sum('returns_cents'), count: rows.reduce((total, row) => total + BigInt(row.sale_count), 0n) };
}

export function localDate(timezone) {
  return new Intl.DateTimeFormat('en-CA', { timeZone: timezone || 'Asia/Jakarta', year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date());
}
