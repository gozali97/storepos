BEGIN;
CREATE TABLE restaurant_modifier_groups (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 product_id UUID NOT NULL,
 name TEXT NOT NULL CHECK(length(btrim(name))>0),
 min_choices INT NOT NULL CHECK(min_choices>=0),
 max_choices INT NOT NULL CHECK(max_choices>=min_choices AND max_choices<=100),
 archived BOOLEAN NOT NULL DEFAULT false,
 version INT NOT NULL DEFAULT 1,
 UNIQUE(business_id,id),
 FOREIGN KEY(business_id,product_id) REFERENCES products(business_id,id)
);
CREATE UNIQUE INDEX modifier_group_names ON restaurant_modifier_groups(business_id,product_id,lower(name)) WHERE NOT archived;
CREATE TABLE restaurant_modifier_options (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 group_id UUID NOT NULL,
 name TEXT NOT NULL CHECK(length(btrim(name))>0),
 price_delta_cents BIGINT NOT NULL CHECK(price_delta_cents>=0),
 active BOOLEAN NOT NULL DEFAULT true,
 UNIQUE(business_id,id),
 FOREIGN KEY(business_id,group_id) REFERENCES restaurant_modifier_groups(business_id,id)
);
CREATE UNIQUE INDEX modifier_option_names ON restaurant_modifier_options(business_id,group_id,lower(name));
COMMIT;
