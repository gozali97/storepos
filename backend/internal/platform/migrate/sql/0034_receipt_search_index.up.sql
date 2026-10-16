BEGIN;
CREATE INDEX goods_receipts_search ON goods_receipts(business_id,branch_id,received_date,created_at DESC,id DESC);
COMMIT;
