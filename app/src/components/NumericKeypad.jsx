import { useState } from 'react';
import { keypadValue } from '../forms.js';
import { formatQty } from '../money.js';

export function NumericKeypad({ value, onChange, decimals = 2, disabled = false, label = 'Keypad numerik' }) {
  const keys = ['7', '8', '9', '4', '5', '6', '1', '2', '3', 'clear', '0', '.', 'backspace'];
  return <div className="numeric-keypad" role="group" aria-label={label}>{keys.map(key => <button type="button" key={key} disabled={disabled || key === '.' && decimals === 0} aria-label={key === 'backspace' ? 'Hapus digit terakhir' : key === 'clear' ? 'Kosongkan angka' : key === '.' ? 'Pemisah desimal' : key} onClick={() => onChange(keypadValue(value, key, decimals))}>{key === 'backspace' ? 'Hapus digit' : key === 'clear' ? 'C' : key === '.' ? ',' : key}</button>)}</div>;
}

export function QuantityKeypad({ line, fractional, disabled, onSave }) {
  const [value, setValue] = useState(formatQty(BigInt(line.qty_milli)).replace(',', '.'));
  return <details className="quantity-keypad"><summary>Keypad kuantitas</summary><label>Jumlah<input type="number" min="0.001" step={fractional ? '0.001' : '1'} value={value} disabled={disabled} onChange={event => setValue(event.target.value)} /></label><NumericKeypad value={value} onChange={setValue} decimals={fractional ? 3 : 0} disabled={disabled} label="Keypad kuantitas produk" /><button type="button" className="primary" disabled={disabled || !value} onClick={() => onSave(value)}>Terapkan kuantitas</button></details>;
}
