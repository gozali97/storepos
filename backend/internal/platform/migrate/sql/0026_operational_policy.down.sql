BEGIN;
ALTER TABLE businesses DROP COLUMN policy_version;
COMMIT;
