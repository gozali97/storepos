BEGIN;
CREATE INDEX sale_drafts_cleanup ON sale_drafts(updated_at,id) WHERE status IN ('active','held','cancelled') AND completed_sale_id IS NULL;
COMMIT;
