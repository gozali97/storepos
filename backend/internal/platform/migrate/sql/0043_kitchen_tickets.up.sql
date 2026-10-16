BEGIN;
CREATE TABLE kitchen_tickets (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 branch_id UUID NOT NULL,
 order_id UUID NOT NULL,
 station_id UUID NOT NULL,
 order_type TEXT NOT NULL,
 table_name TEXT,
 created_by UUID NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(business_id,id),
 FOREIGN KEY(business_id,order_id) REFERENCES restaurant_orders(business_id,id),
 FOREIGN KEY(business_id,branch_id,station_id) REFERENCES kitchen_stations(business_id,branch_id,id)
);
CREATE TABLE kitchen_ticket_items (
 business_id UUID NOT NULL,
 ticket_id UUID NOT NULL,
 line_id UUID NOT NULL,
 snapshot JSONB NOT NULL,
 PRIMARY KEY(ticket_id,line_id),
 UNIQUE(business_id,line_id),
 FOREIGN KEY(business_id,ticket_id) REFERENCES kitchen_tickets(business_id,id),
 FOREIGN KEY(business_id,line_id) REFERENCES restaurant_order_lines(business_id,id)
);
CREATE INDEX kitchen_tickets_station ON kitchen_tickets(business_id,branch_id,station_id,created_at,id);
COMMIT;
