import { test } from 'node:test';
import assert from 'node:assert/strict';
import { receiptDraftBody, receiptEditableLines } from './purchasing.js';

test('draft penerimaan roundtrip menjaga qty pecahan dan harga tanpa float', () => {
 const receipt={lines:[{variant_id:'v',unit_id:'u',qty_milli:'1250',purchase_price_cents:'9007199254740993'}]};
 const editable=receiptEditableLines(receipt);
 const body=receiptDraftBody({received_date:'2026-10-14',invoice_number:' INV ',supplier_id:''},editable);
 assert.deepEqual(body,{received_date:'2026-10-14',invoice_number:'INV',supplier_id:null,lines:[{unit_id:'u',qty_milli:'1250',purchase_price_cents:'9007199254740993'}]});
});
test('draft menolak jumlah baris tidak valid, satuan kosong dan qty nol',()=>{
 assert.throws(()=>receiptDraftBody({},[]));
 assert.throws(()=>receiptDraftBody({},[{unit_id:'',qty:'1',price:'2'}]));
 assert.throws(()=>receiptDraftBody({},[{unit_id:'u',qty:'0',price:'2'}]));
 assert.throws(()=>receiptDraftBody({},Array.from({length:501},()=>({unit_id:'u',qty:'1',price:'2'}))));
});
