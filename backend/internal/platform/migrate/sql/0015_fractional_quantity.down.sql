BEGIN;
ALTER TABLE sale_lines DROP COLUMN allow_fractional;
ALTER TABLE variants DROP COLUMN allow_fractional;
COMMIT;
