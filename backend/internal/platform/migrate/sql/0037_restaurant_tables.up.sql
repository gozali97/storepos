BEGIN;
CREATE TABLE restaurant_areas (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 branch_id UUID NOT NULL,
 name TEXT NOT NULL CHECK(length(btrim(name))>0),
 archived BOOLEAN NOT NULL DEFAULT false,
 version INT NOT NULL DEFAULT 1,
 UNIQUE(business_id,branch_id,id),
 FOREIGN KEY(business_id,branch_id) REFERENCES branches(business_id,id)
);
CREATE UNIQUE INDEX restaurant_area_names ON restaurant_areas(business_id,branch_id,lower(name)) WHERE NOT archived;
CREATE TABLE restaurant_tables (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 branch_id UUID NOT NULL,
 area_id UUID NOT NULL,
 name TEXT NOT NULL CHECK(length(btrim(name))>0),
 capacity INT NOT NULL CHECK(capacity>0 AND capacity<=1000),
 archived BOOLEAN NOT NULL DEFAULT false,
 version INT NOT NULL DEFAULT 1,
 UNIQUE(business_id,branch_id,id),
 FOREIGN KEY(business_id,branch_id,area_id) REFERENCES restaurant_areas(business_id,branch_id,id)
);
CREATE UNIQUE INDEX restaurant_table_names ON restaurant_tables(business_id,branch_id,lower(name)) WHERE NOT archived;
COMMIT;
