BEGIN;
-- PRD §13.1/§13.2: transisi status selain yang tercantum ditolak (INVALID_STATE_TRANSITION).
-- Dipasang sebagai trigger agar berlaku untuk semua jalur tulis, bukan hanya handler HTTP.
-- SQLSTATE 'ST001' dipetakan httpx.WriteError menjadi 409 INVALID_STATE_TRANSITION.
CREATE OR REPLACE FUNCTION enforce_status_transition() RETURNS trigger AS $$
DECLARE
    allowed boolean;
BEGIN
    IF NEW.status IS NOT DISTINCT FROM OLD.status THEN
        RETURN NEW;
    END IF;
    allowed := CASE TG_TABLE_NAME
        WHEN 'sales'             THEN OLD.status = 'completed' AND NEW.status = 'voided'
        WHEN 'payments'          THEN OLD.status = 'confirmed' AND NEW.status = 'reversed'
        WHEN 'restaurant_orders' THEN OLD.status = 'open' AND NEW.status IN ('completed', 'cancelled')
        WHEN 'sale_drafts'       THEN
            (OLD.status IN ('active', 'held') AND NEW.status IN ('active', 'held', 'cancelled', 'completed'))
        ELSE false
    END;
    IF NOT allowed THEN
        RAISE EXCEPTION 'transisi status % -> % tidak diizinkan pada %', OLD.status, NEW.status, TG_TABLE_NAME
            USING ERRCODE = 'ST001';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER sales_status_transition BEFORE UPDATE OF status ON sales
    FOR EACH ROW EXECUTE FUNCTION enforce_status_transition();
CREATE TRIGGER payments_status_transition BEFORE UPDATE OF status ON payments
    FOR EACH ROW EXECUTE FUNCTION enforce_status_transition();
CREATE TRIGGER restaurant_orders_status_transition BEFORE UPDATE OF status ON restaurant_orders
    FOR EACH ROW EXECUTE FUNCTION enforce_status_transition();
CREATE TRIGGER sale_drafts_status_transition BEFORE UPDATE OF status ON sale_drafts
    FOR EACH ROW EXECUTE FUNCTION enforce_status_transition();

-- Status retur hanya maju: none -> partial -> full (atribut terpisah dari status transaksi).
CREATE OR REPLACE FUNCTION enforce_return_status_transition() RETURNS trigger AS $$
BEGIN
    IF NEW.return_status IS NOT DISTINCT FROM OLD.return_status THEN
        RETURN NEW;
    END IF;
    IF NOT (OLD.return_status = 'none' AND NEW.return_status IN ('partial', 'full')
            OR OLD.return_status = 'partial' AND NEW.return_status = 'full') THEN
        RAISE EXCEPTION 'transisi status retur % -> % tidak diizinkan', OLD.return_status, NEW.return_status
            USING ERRCODE = 'ST001';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER sales_return_status_transition BEFORE UPDATE OF return_status ON sales
    FOR EACH ROW EXECUTE FUNCTION enforce_return_status_transition();
COMMIT;
