BEGIN;
ALTER TABLE restaurant_orders ADD COLUMN contact_name TEXT NOT NULL DEFAULT '' CHECK(char_length(contact_name)<=100), ADD COLUMN contact_phone TEXT NOT NULL DEFAULT '' CHECK(char_length(contact_phone)<=32), ADD COLUMN queue_label TEXT NOT NULL DEFAULT '' CHECK(char_length(queue_label)<=40);
COMMIT;
