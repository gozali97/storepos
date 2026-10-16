package reporting

import (
	"context"

	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

type CleanupResult struct{ Drafts, IdempotencyKeys int64 }

// Cleanup handles bounded batches and skips rows in use by active requests.
// Never delete completed drafts/sales, stock history, or append-only audit.
func Cleanup(ctx context.Context, pool *database.Pool) (CleanupResult, error) {
	var result CleanupResult
	err := pool.InTx(ctx, func(ctx context.Context, tx database.Tx) error {
		result = CleanupResult{}
		rows, err := tx.Query(ctx, `WITH candidates AS (
   SELECT id FROM sale_drafts WHERE status IN ('active','held','cancelled') AND completed_sale_id IS NULL
   AND updated_at < now()-interval '30 days' ORDER BY updated_at,id LIMIT 500 FOR UPDATE SKIP LOCKED
  ) DELETE FROM sale_drafts d USING candidates c WHERE d.id=c.id RETURNING d.id,d.business_id,d.branch_id,d.register_id`)
		if err != nil {
			return err
		}
		type draft struct{ id, biz, branch, reg idkit.ID }
		var deleted []draft
		for rows.Next() {
			var d draft
			if err := rows.Scan(&d.id, &d.biz, &d.branch, &d.reg); err != nil {
				rows.Close()
				return err
			}
			deleted = append(deleted, d)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, d := range deleted {
			if err := audit.Write(ctx, tx, audit.Entry{BusinessID: d.biz, BranchID: &d.branch, RegisterID: &d.reg, Action: "draft.expired", EntityType: "draft", EntityID: d.id.String(), Reason: "inactive_more_than_30_days"}); err != nil {
				return err
			}
		}
		result.Drafts = int64(len(deleted))
		tag, err := tx.Exec(ctx, `WITH candidates AS (
   SELECT business_id,key FROM idempotency_keys WHERE expires_at<now() AND created_at<now()-interval '72 hours'
   ORDER BY expires_at,business_id,key LIMIT 500 FOR UPDATE SKIP LOCKED
  ) DELETE FROM idempotency_keys k USING candidates c WHERE k.business_id=c.business_id AND k.key=c.key`)
		if err != nil {
			return err
		}
		result.IdempotencyKeys = tag.RowsAffected()
		return nil
	})
	return result, err
}
