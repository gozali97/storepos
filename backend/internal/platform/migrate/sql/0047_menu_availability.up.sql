BEGIN;
CREATE TABLE restaurant_menu_availability (
 business_id UUID NOT NULL,
 branch_id UUID NOT NULL,
 variant_id UUID NOT NULL,
 sold_out BOOLEAN NOT NULL,
 version INT NOT NULL DEFAULT 1,
 updated_by UUID NOT NULL REFERENCES users(id),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(business_id,branch_id,variant_id),
 FOREIGN KEY(business_id,branch_id) REFERENCES branches(business_id,id),
 FOREIGN KEY(business_id,variant_id) REFERENCES variants(business_id,id)
);
COMMIT;
