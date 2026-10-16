import { test } from 'node:test';
import assert from 'node:assert/strict';
import { normalizePhone, validateLead } from './forms.js';
import { analyticsPayload } from './analytics.js';

test('nomor Indonesia dinormalisasi tanpa menerima teks dan negara lain', () => {
  for (const value of ['0812-3456-7890', '62 812 3456 7890', '+62 (812) 3456.7890']) assert.equal(normalizePhone(value), '+6281234567890');
  for (const value of ['+12025550123', 'abc081234567890', '081', '+628123456789012345', '62812+34567890']) assert.equal(normalizePhone(value), '');
});

test('validasi consent, panjang field dan format email tanpa kehilangan isian', () => {
  const data = { name: 'Pemilik demo', business_name: 'Toko demo', phone: '081234567890', business_type: 'retail', city: 'Jakarta', consent: true };
  assert.deepEqual(validateLead(data), {});
  const before = { ...data };
  const invalid = validateLead({ ...data, consent: false, notes: 'a'.repeat(1001), email: 'invalid', city: 'A' });
  assert.deepEqual(Object.keys(invalid).sort(), ['city', 'consent', 'email', 'notes']);
  assert.deepEqual(data, before);
});

test('payload analytics menghapus seluruh field pribadi dan menolak event asing', () => {
  const pii = { name: 'Pemilik demo', phone: '+6281234567890', email: 'private@example.test', business_name: 'Rahasia', city: 'Lokasi rahasia', notes: 'Catatan pribadi' };
  assert.deepEqual(analyticsPayload('demo_form_submit_success', { ...pii, business_type: 'retail', branch_bucket: '1', has_email: true }), { event: 'demo_form_submit_success', properties: { business_type: 'retail', branch_bucket: '1', has_email: true } });
  for (const event of ['lp_view', 'lp_cta_click', 'lp_business_tab_select', 'lp_demo_step_view', 'lp_faq_open', 'demo_form_start', 'demo_form_field_error', 'demo_form_submit_attempt', 'demo_form_submit_error']) assert.deepEqual(analyticsPayload(event, pii).properties, {});
  assert.equal(analyticsPayload('unknown', pii), null);
});
