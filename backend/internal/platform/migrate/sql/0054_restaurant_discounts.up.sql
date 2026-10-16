BEGIN;
ALTER TABLE restaurant_orders ADD COLUMN discount_kind TEXT NOT NULL DEFAULT 'none' CHECK(discount_kind IN ('none','amount','percent')),
 ADD COLUMN discount_value BIGINT NOT NULL DEFAULT 0 CHECK(discount_value>=0),
 ADD COLUMN discount_reason TEXT NOT NULL DEFAULT '' CHECK(char_length(discount_reason)<=200);
ALTER TABLE restaurant_order_lines ADD COLUMN discount_kind TEXT NOT NULL DEFAULT 'none' CHECK(discount_kind IN ('none','amount','percent')),
 ADD COLUMN discount_value BIGINT NOT NULL DEFAULT 0 CHECK(discount_value>=0);
COMMIT;
