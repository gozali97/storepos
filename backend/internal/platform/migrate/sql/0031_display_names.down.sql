BEGIN;
ALTER TABLE users DROP COLUMN display_name,DROP COLUMN profile_version;
COMMIT;
