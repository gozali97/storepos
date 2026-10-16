BEGIN;
ALTER TABLE businesses ADD COLUMN org_version INT NOT NULL DEFAULT 1;
UPDATE businesses b SET first_sale_completed_at=s.first_sale FROM (SELECT business_id,MIN(completed_at) AS first_sale FROM sales GROUP BY business_id) s WHERE b.id=s.business_id AND b.first_sale_completed_at IS NULL;
CREATE FUNCTION businesses_lock_type() RETURNS trigger AS $$
BEGIN
 IF OLD.first_sale_completed_at IS NOT NULL AND (NEW.business_type<>OLD.business_type OR NEW.enabled_modules IS DISTINCT FROM OLD.enabled_modules OR NEW.first_sale_completed_at IS NULL) THEN
 RAISE EXCEPTION 'jenis usaha/modul terkunci setelah transaksi pertama';
 END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER businesses_type_lock BEFORE UPDATE ON businesses FOR EACH ROW EXECUTE FUNCTION businesses_lock_type();
COMMIT;
