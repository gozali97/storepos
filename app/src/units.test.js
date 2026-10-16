import { test } from 'node:test';
import assert from 'node:assert/strict';
import { resolveDefaultUnit } from './units.js';

test('default dasar null, pembelian boleh tanpa harga dan penjualan harus punya harga', () => {
 const units=[{unit_id:'box',sale_price_cents:null},{unit_id:'pack',sale_price_cents:'1234567890123'}];
 assert.equal(resolveDefaultUnit(units,{},'sale'),null);
 assert.equal(resolveDefaultUnit(units,{default_purchase_unit_id:'box'},'purchase'),units[0]);
 assert.equal(resolveDefaultUnit(units,{default_sale_unit_id:'pack'},'sale'),units[1]);
 assert.throws(()=>resolveDefaultUnit(units,{default_sale_unit_id:'box'},'sale'));
});
test('default hilang atau arsip tidak diam-diam beralih ke satuan dasar',()=>{
 assert.throws(()=>resolveDefaultUnit([],{default_sale_unit_id:'missing'},'sale'));
 assert.throws(()=>resolveDefaultUnit([{unit_id:'x',archived:true}],{default_purchase_unit_id:'x'},'purchase'));
});
