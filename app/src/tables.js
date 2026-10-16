// Logika murni grid meja restoran (RST-002): pengelompokan per area dan status turunan dari server.

const LABELS = { AVAILABLE: 'Kosong', OCCUPIED: 'Terisi', NEEDS_CLEANING: 'Perlu dibersihkan' };
const TONES = { AVAILABLE: 'ok', OCCUPIED: 'warn', NEEDS_CLEANING: 'danger' };

export function tableStatusLabel(status) {
  return LABELS[status] ?? status;
}

export function tableStatusTone(status) {
  return TONES[status] ?? 'ok';
}

/** Meja aktif dikelompokkan per area, urut nama area lalu nama meja (numerik wajar: "2" < "10"). */
export function groupByArea(tables) {
  const collator = new Intl.Collator('id', { numeric: true });
  const areas = new Map();
  for (const table of tables) {
    if (table.archived) continue;
    if (!areas.has(table.area_id)) areas.set(table.area_id, { id: table.area_id, name: table.area_name, tables: [] });
    areas.get(table.area_id).tables.push(table);
  }
  return [...areas.values()]
    .map(area => ({ ...area, tables: area.tables.sort((a, b) => collator.compare(a.name, b.name)) }))
    .sort((a, b) => collator.compare(a.name, b.name));
}

/** Ringkasan jumlah meja per status untuk kepala halaman. */
export function tableSummary(tables) {
  const summary = { AVAILABLE: 0, OCCUPIED: 0, NEEDS_CLEANING: 0 };
  for (const table of tables) if (!table.archived && table.status in summary) summary[table.status] += 1;
  return summary;
}

/** Hanya meja yang perlu dibersihkan yang boleh ditandai bersih; server tetap memvalidasi. */
export function canClean(table) {
  return table.status === 'NEEDS_CLEANING';
}

export function filterTables(tables, { area = '', status = '', query = '' } = {}) {
  const search = query.trim().toLocaleLowerCase('id');
  return tables.filter(table => !table.archived && (!area || table.area_id === area) && (!status || table.status === status)
    && (!search || `${table.name} ${table.area_name}`.toLocaleLowerCase('id').includes(search)));
}
