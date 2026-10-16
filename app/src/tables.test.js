import { test } from 'node:test';
import assert from 'node:assert/strict';
import { filterTables, groupByArea, tableSummary, canClean, tableStatusLabel, tableStatusTone } from './tables.js';

const t = (area_id, area_name, name, status, extra = {}) => ({ table_id: `${area_id}-${name}`, area_id, area_name, name, status, archived: false, ...extra });

test('meja dikelompokkan per area dengan urutan numerik wajar', () => {
  const groups = groupByArea([t('b', 'Teras', '10', 'AVAILABLE'), t('a', 'Indoor', '2', 'AVAILABLE'), t('b', 'Teras', '2', 'AVAILABLE'), t('a', 'Indoor', '10', 'OCCUPIED')]);
  assert.deepEqual(groups.map(g => g.name), ['Indoor', 'Teras']);
  assert.deepEqual(groups[0].tables.map(x => x.name), ['2', '10']);
});

test('meja terarsip tidak ditampilkan dan tidak dihitung', () => {
  const tables = [t('a', 'Indoor', '1', 'AVAILABLE'), t('a', 'Indoor', '2', 'OCCUPIED', { archived: true })];
  assert.equal(groupByArea(tables)[0].tables.length, 1);
  assert.deepEqual(tableSummary(tables), { AVAILABLE: 1, OCCUPIED: 0, NEEDS_CLEANING: 0 });
});

test('ringkasan menghitung tiap status turunan server', () => {
  const summary = tableSummary([t('a', 'I', '1', 'AVAILABLE'), t('a', 'I', '2', 'OCCUPIED'), t('a', 'I', '3', 'NEEDS_CLEANING'), t('a', 'I', '4', 'NEEDS_CLEANING')]);
  assert.deepEqual(summary, { AVAILABLE: 1, OCCUPIED: 1, NEEDS_CLEANING: 2 });
});

test('hanya meja perlu dibersihkan yang dapat ditandai bersih', () => {
  assert.equal(canClean({ status: 'NEEDS_CLEANING' }), true);
  assert.equal(canClean({ status: 'OCCUPIED' }), false);
  assert.equal(canClean({ status: 'AVAILABLE' }), false);
});

test('label dan nada status tidak mengandalkan warna saja', () => {
  assert.equal(tableStatusLabel('NEEDS_CLEANING'), 'Perlu dibersihkan');
  assert.equal(tableStatusTone('OCCUPIED'), 'warn');
  assert.equal(tableStatusLabel('LAIN'), 'LAIN');
});

test('filter menggabungkan area, status dan pencarian tanpa menyertakan arsip', () => {
  const tables = [t('a', 'Indoor', 'VIP 1', 'AVAILABLE'), t('a', 'Indoor', 'VIP 2', 'OCCUPIED'), t('b', 'Teras', 'VIP 3', 'AVAILABLE'), t('a', 'Indoor', 'VIP 4', 'AVAILABLE', { archived: true })];
  assert.equal(filterTables(tables).length, 3);
  assert.deepEqual(filterTables(tables, { area: 'a', status: 'AVAILABLE', query: '  vip  ' }).map(table => table.name), ['VIP 1']);
  assert.equal(filterTables(tables, { query: 'INDOOR' }).length, 2);
  assert.equal(filterTables(tables, { area: 'b', status: 'OCCUPIED' }).length, 0);
});
