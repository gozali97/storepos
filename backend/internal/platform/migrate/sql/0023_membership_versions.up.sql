BEGIN;
ALTER TABLE memberships ADD COLUMN version INT NOT NULL DEFAULT 1;
CREATE UNIQUE INDEX memberships_business_id_id ON memberships(business_id,id);
ALTER TABLE membership_branches ADD CONSTRAINT membership_branches_tenant_fk FOREIGN KEY(business_id,membership_id) REFERENCES memberships(business_id,id) ON DELETE CASCADE;
COMMIT;
