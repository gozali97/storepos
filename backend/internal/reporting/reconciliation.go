package reporting

import (
	"context"
	"log/slog"
	"time"

	"github.com/storepos/storepos/internal/platform/database"
)

// UNION is one statement/snapshot: concurrent committed sales cannot cause false discrepancies.
// RETURN_DAMAGED records physical returned goods but does not change saleable stock.
const reconcileSQL = `WITH stock AS (
 SELECT business_id,branch_id,variant_id,SUM(qty_milli) AS qty
 FROM stock_movements WHERE kind<>'RETURN_DAMAGED' GROUP BY business_id,branch_id,variant_id
), paid AS (
 SELECT business_id,sale_id,SUM(amount_cents) FILTER(WHERE status='confirmed') AS amount
 FROM payments GROUP BY business_id,sale_id
)
SELECT COALESCE(b.business_id,m.business_id)::text AS business_id,'stock' AS kind,
 COALESCE(b.branch_id,m.branch_id)::text||':'||COALESCE(b.variant_id,m.variant_id)::text AS entity_key,
 COALESCE(m.qty,0)::text AS expected_value,COALESCE(b.qty_milli,0)::text AS actual_value
FROM inventory_balances b FULL JOIN stock m ON m.business_id=b.business_id AND m.branch_id=b.branch_id AND m.variant_id=b.variant_id
WHERE COALESCE(b.qty_milli,0)<>COALESCE(m.qty,0)
UNION ALL
SELECT s.business_id::text,'payment',s.id::text,
 CASE WHEN s.status='completed' THEN s.total_cents ELSE 0 END::text,COALESCE(p.amount,0)::text
FROM sales s LEFT JOIN paid p ON p.business_id=s.business_id AND p.sale_id=s.id
WHERE COALESCE(p.amount,0)<>CASE WHEN s.status='completed' THEN s.total_cents ELSE 0 END
UNION ALL
SELECT COALESCE(b.business_id,r.business_id)::text,'reservation',
 COALESCE(b.branch_id,r.branch_id)::text||':'||COALESCE(b.variant_id,r.variant_id)::text,
 COALESCE(r.qty,0)::text,COALESCE(b.reserved_milli,0)::text
FROM inventory_balances b FULL JOIN (
 SELECT business_id,branch_id,variant_id,SUM(qty_milli) AS qty FROM restaurant_stock_reservations WHERE status='active' GROUP BY business_id,branch_id,variant_id
) r ON r.business_id=b.business_id AND r.branch_id=b.branch_id AND r.variant_id=b.variant_id
WHERE COALESCE(r.qty,0)<>COALESCE(b.reserved_milli,0) OR COALESCE(b.reserved_milli,0)>COALESCE(b.qty_milli,0)`

type Finding struct {
	BusinessID string `json:"business_id"`
	Kind       string `json:"kind"`
	EntityKey  string `json:"entity_key"`
	Expected   string `json:"expected_value"`
	Actual     string `json:"actual_value"`
}

// Check only reads. No automatic repair of financial or inventory data.
func Check(ctx context.Context, db database.Getter) ([]Finding, error) {
	rows, err := db.Query(ctx, reconcileSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Finding{}
	for rows.Next() {
		var f Finding
		if err := rows.Scan(&f.BusinessID, &f.Kind, &f.EntityKey, &f.Expected, &f.Actual); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// RunDaily's unique date claim prevents duplicate workers. Claim and findings commit together;
// a crash rolls back the claim so the next invocation can retry. Schedule is UTC.
func RunDaily(ctx context.Context, pool *database.Pool, now time.Time) (bool, int64, error) {
	date := now.UTC().Format("2006-01-02")
	var ran bool
	var n int64
	err := pool.InTx(ctx, func(ctx context.Context, tx database.Tx) error {
		ran = false
		n = 0
		tag, err := tx.Exec(ctx, `INSERT INTO reconciliation_runs(run_date) VALUES($1) ON CONFLICT DO NOTHING`, date)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		// The insert-select has a single MVCC snapshot for both invariants.
		tag, err = tx.Exec(ctx, `INSERT INTO reconciliation_findings(id,run_date,business_id,kind,entity_key,expected_value,actual_value)
 SELECT gen_random_uuid(),$1,business_id::uuid,kind,entity_key,expected_value::numeric,actual_value::numeric FROM (`+reconcileSQL+`) findings`, date)
		if err != nil {
			return err
		}
		n = tag.RowsAffected()
		if _, err := tx.Exec(ctx, `UPDATE reconciliation_runs SET completed_at=clock_timestamp(),finding_count=$2 WHERE run_date=$1`, date, n); err != nil {
			return err
		}
		ran = true
		return nil
	})
	return ran, n, err
}

// Worker polls export jobs every five seconds and daily reconciliation claims on startup/poll. Alert logs contain counts, no customer PII.
// shortcut: alerts via structured error logs; connect monitoring/email when the operational provider is selected.
func Worker(ctx context.Context, pool *database.Pool, log *slog.Logger) error {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		ran, n, err := RunDaily(runCtx, pool, time.Now())
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Error("reconciliation failed", "err", err)
		} else if ran {
			if n > 0 {
				log.Error("reconciliation discrepancy", "findings", n)
			} else {
				log.Info("reconciliation completed", "findings", n)
			}
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(ctx, time.Minute)
		cleaned, cleanupErr := Cleanup(cleanupCtx, pool)
		cleanupCancel()
		if cleanupErr != nil && ctx.Err() == nil {
			log.Error("cleanup failed", "err", cleanupErr)
		} else if cleaned.Drafts > 0 || cleaned.IdempotencyKeys > 0 {
			log.Info("cleanup completed", "drafts", cleaned.Drafts, "idempotency_keys", cleaned.IdempotencyKeys)
		}
		exportCtx, exportCancel := context.WithTimeout(ctx, 10*time.Minute)
		_, exportErr := ProcessExport(exportCtx, pool)
		exportCancel()
		if exportErr != nil && ctx.Err() == nil {
			log.Error("report export failed", "err", exportErr)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}
