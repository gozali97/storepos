BEGIN;
ALTER TABLE invitations DROP COLUMN revoked_at;
DROP INDEX invitations_token_hash_unique;
COMMIT;
