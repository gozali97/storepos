import { test } from 'node:test';
import assert from 'node:assert/strict';
import { rupiahToCents, keypadValue } from './forms.js';
test('input Rupiah dikonversi tanpa floating point', () => {
  assert.equal(rupiahToCents('25000.5'), '2500050');
  assert.equal(rupiahToCents('9007199254740993.01'), '900719925474099301');
  assert.equal(rupiahToCents('0'), '0');
  for (const value of ['-1', 'NaN', '1.234', '', '1e3']) assert.throws(() => rupiahToCents(value));
});

test('keypad menjaga presisi, batas desimal, hapus digit dan nol awal', () => {
  let value = '';
  for (const key of ['0', '0', '2', '5', '.', '5', '0', '9']) value = keypadValue(value, key);
  assert.equal(value, '25.50');
  assert.equal(rupiahToCents(value), '2550');
  assert.equal(keypadValue(value, '.'), value);
  assert.equal(keypadValue(value, 'backspace'), '25.5');
  assert.equal(keypadValue(value, 'clear'), '');
  assert.equal(keypadValue('', '.', 0), '');
  assert.equal(keypadValue('0.37', '5', 3), '0.375');
  assert.equal(keypadValue('9007199254740993', '1'), '90071992547409931');
});
