BEGIN;
CREATE UNIQUE INDEX invitations_token_hash_unique ON invitations(token_hash);
ALTER TABLE invitations ADD COLUMN revoked_at TIMESTAMPTZ;
COMMIT;
