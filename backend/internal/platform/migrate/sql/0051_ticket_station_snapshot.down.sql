BEGIN;
ALTER TABLE kitchen_tickets DROP COLUMN station_name_snapshot;
COMMIT;
