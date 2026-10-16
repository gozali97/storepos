BEGIN;
CREATE TABLE restaurant_orders (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 branch_id UUID NOT NULL,
 table_id UUID,
 order_type TEXT NOT NULL CHECK(order_type IN ('DINE_IN','TAKEAWAY','DELIVERY_MANUAL')),
 status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','completed','cancelled')),
 payment_status TEXT NOT NULL DEFAULT 'unpaid' CHECK(payment_status IN ('unpaid','paid')),
 version INT NOT NULL DEFAULT 1,
 note TEXT NOT NULL DEFAULT '' CHECK(char_length(note)<=200),
 channel TEXT NOT NULL DEFAULT '' CHECK(char_length(channel)<=100),
 delivery_fee_cents BIGINT NOT NULL DEFAULT 0 CHECK(delivery_fee_cents>=0),
 created_by UUID NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 sale_id UUID,
 UNIQUE(business_id,id),
 FOREIGN KEY(business_id,branch_id) REFERENCES branches(business_id,id),
 FOREIGN KEY(business_id,branch_id,table_id) REFERENCES restaurant_tables(business_id,branch_id,id),
 FOREIGN KEY(business_id,sale_id) REFERENCES sales(business_id,id),
 CHECK((order_type='DINE_IN' AND table_id IS NOT NULL) OR (order_type<>'DINE_IN' AND table_id IS NULL))
);
CREATE UNIQUE INDEX restaurant_orders_open_table ON restaurant_orders(business_id,table_id) WHERE status='open' AND table_id IS NOT NULL;
CREATE INDEX restaurant_orders_branch ON restaurant_orders(business_id,branch_id,status,created_at,id);
COMMIT;
