BEGIN;
ALTER TABLE variants ADD COLUMN allow_fractional BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE sale_lines ADD COLUMN allow_fractional BOOLEAN NOT NULL DEFAULT false;
-- Preserve historical fractional sales for correct partial returns.
UPDATE sale_lines SET allow_fractional=true WHERE qty_milli % 1000 <> 0;
COMMIT;
