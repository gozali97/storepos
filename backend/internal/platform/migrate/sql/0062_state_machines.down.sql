BEGIN;
DROP TRIGGER sales_return_status_transition ON sales;
DROP TRIGGER sale_drafts_status_transition ON sale_drafts;
DROP TRIGGER restaurant_orders_status_transition ON restaurant_orders;
DROP TRIGGER payments_status_transition ON payments;
DROP TRIGGER sales_status_transition ON sales;
DROP FUNCTION enforce_return_status_transition();
DROP FUNCTION enforce_status_transition();
COMMIT;
