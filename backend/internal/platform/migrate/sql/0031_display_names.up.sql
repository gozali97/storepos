BEGIN;
ALTER TABLE users ADD COLUMN display_name TEXT NOT NULL DEFAULT '' CHECK(char_length(display_name)<=200), ADD COLUMN profile_version INT NOT NULL DEFAULT 1;
COMMIT;
