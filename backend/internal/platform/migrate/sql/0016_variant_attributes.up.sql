BEGIN;
ALTER TABLE variants ADD COLUMN attributes JSONB NOT NULL DEFAULT '{}'::jsonb CHECK(jsonb_typeof(attributes)='object');
COMMIT;
