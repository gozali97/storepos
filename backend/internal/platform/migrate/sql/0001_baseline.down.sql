-- Rollback baseline (0001).
-- Hati-hati: ini menghapus seluruh skema fondasi. Hanya untuk dev/test.

BEGIN;

-- Catatan: tabel schema_migrations tidak dihapus di sini; runner migrasi
-- yang mengelolanya dan mencatat versi ini sebagai belum diterapkan.
DROP TABLE IF EXISTS document_counters;
DROP TABLE IF EXISTS outbox_jobs;
DROP TABLE IF EXISTS idempotency_keys;

DROP TRIGGER IF EXISTS audit_logs_no_delete ON audit_logs;
DROP TRIGGER IF EXISTS audit_logs_no_update ON audit_logs;
DROP FUNCTION IF EXISTS audit_logs_block_mutation();
DROP TABLE IF EXISTS audit_logs;

DROP TABLE IF EXISTS invitations;
DROP TABLE IF EXISTS membership_branches;
DROP TABLE IF EXISTS memberships;
DROP TABLE IF EXISTS registers;
DROP INDEX IF EXISTS branches_business_id_id_key;
DROP TABLE IF EXISTS branches;
DROP TABLE IF EXISTS businesses;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

COMMIT;
