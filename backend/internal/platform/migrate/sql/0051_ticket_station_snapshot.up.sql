BEGIN;
ALTER TABLE kitchen_tickets ADD COLUMN station_name_snapshot TEXT;
-- Old station names remain unknown, not silently reconstructed from current master.
COMMIT;
