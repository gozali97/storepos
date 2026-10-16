BEGIN;
ALTER TABLE membership_branches DROP CONSTRAINT membership_branches_tenant_fk;
DROP INDEX memberships_business_id_id;
ALTER TABLE memberships DROP COLUMN version;
COMMIT;
