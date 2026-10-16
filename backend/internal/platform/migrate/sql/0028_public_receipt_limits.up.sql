BEGIN;
CREATE TABLE public_receipt_limits (
 key_hash TEXT PRIMARY KEY,
 window_start TIMESTAMPTZ NOT NULL,
 requests INT NOT NULL CHECK(requests>0)
);
COMMIT;
