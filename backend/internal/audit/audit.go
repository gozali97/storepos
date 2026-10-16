// Package audit menulis jejak audit append-only (PRD CORE-042).
// Penulisan harus dilakukan pada transaksi yang sama dengan perubahan datanya.
package audit

import (
	"context"
	"fmt"

	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
)

// Entry adalah satu catatan audit. Before/After dapat nil.
type Entry struct {
	BusinessID  idkit.ID
	BranchID    *idkit.ID
	RegisterID  *idkit.ID
	ActorUserID idkit.ID
	ApprovedBy  *idkit.ID
	Action      string // contoh: "sale.completed"
	EntityType  string
	EntityID    string
	Before      any
	After       any
	Reason      string
}

// Write menyisipkan catatan audit.
func Write(ctx context.Context, db database.Getter, e Entry) error {
	var reason any
	if e.Reason != "" {
		reason = e.Reason
	}
	var business, actor, ip, userAgent any
	if !e.BusinessID.IsZero() {
		business = e.BusinessID
	}
	if !e.ActorUserID.IsZero() {
		actor = e.ActorUserID
	}
	if metadata, ok := ctx.Value(requestKey{}).(RequestMetadata); ok {
		if metadata.IP != "" {
			ip = metadata.IP
		}
		if metadata.UserAgent != "" {
			userAgent = metadata.UserAgent
		}
	}
	_, err := db.Exec(ctx, `
INSERT INTO audit_logs (id, business_id, branch_id, register_id, actor_user_id, approved_by_user_id,
                        action, entity_type, entity_id, before_jsonb, after_jsonb, reason,ip,user_agent)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		idkit.New(), business, e.BranchID, e.RegisterID, actor, e.ApprovedBy,
		e.Action, e.EntityType, e.EntityID, e.Before, e.After, reason, ip, userAgent)
	if err != nil {
		return fmt.Errorf("audit: tulis %s: %w", e.Action, err)
	}
	return nil
}
