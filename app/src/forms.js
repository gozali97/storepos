export function rupiahToCents(value) {
  const match = /^(\d+)(?:\.(\d{1,2}))?$/.exec(String(value).trim());
  if (!match) throw new Error('Nominal Rupiah harus positif dengan maksimal dua desimal.');
  return (BigInt(match[1]) * 100n + BigInt((match[2] ?? '').padEnd(2, '0'))).toString();
}

export function keypadValue(value, key, decimals = 2) {
  if (key === 'clear') return '';
  if (key === 'backspace') return value.slice(0, -1);
  if (key === '.') return decimals > 0 && !value.includes('.') ? `${value || '0'}.` : value;
  if (!/^\d$/.test(key)) return value;
  const fraction = value.split('.')[1];
  if (fraction != null && fraction.length >= decimals) return value;
  return value === '0' ? key : value + key;
}
