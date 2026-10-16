BEGIN;
CREATE TABLE kitchen_stations (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 branch_id UUID NOT NULL,
 name TEXT NOT NULL CHECK(length(btrim(name))>0),
 archived BOOLEAN NOT NULL DEFAULT false,
 version INT NOT NULL DEFAULT 1,
 UNIQUE(business_id,branch_id,id),
 FOREIGN KEY(business_id,branch_id) REFERENCES branches(business_id,id)
);
CREATE UNIQUE INDEX kitchen_station_names ON kitchen_stations(business_id,branch_id,lower(name)) WHERE NOT archived;
CREATE TABLE kitchen_category_routes (
 business_id UUID NOT NULL,
 branch_id UUID NOT NULL,
 category_id UUID NOT NULL,
 station_id UUID NOT NULL,
 version INT NOT NULL DEFAULT 1,
 PRIMARY KEY(business_id,branch_id,category_id),
 FOREIGN KEY(business_id,category_id) REFERENCES categories(business_id,id),
 FOREIGN KEY(business_id,branch_id,station_id) REFERENCES kitchen_stations(business_id,branch_id,id)
);
COMMIT;
