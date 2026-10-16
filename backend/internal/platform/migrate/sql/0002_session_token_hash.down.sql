-- Rollback 0002.

BEGIN;

DROP INDEX IF EXISTS sessions_user_active_idx;
DROP INDEX IF EXISTS sessions_expires_idx;
DROP INDEX IF EXISTS sessions_token_hash_key;

ALTER TABLE sessions DROP COLUMN IF EXISTS token_hash;

COMMIT;
