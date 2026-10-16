package reporting

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/audit"
	"github.com/storepos/storepos/internal/org"
	"github.com/storepos/storepos/internal/platform/authz"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/platform/tenant"
)

func reportLimit(f PaymentFilter) int {
	if f.Export {
		return 0
	}
	return 100001
}

type ExportParameters struct {
	Filter      PaymentFilter
	Group       string
	StockFilter string
}

var ErrExportPermission = errors.New("EXPORT_PERMISSION_REVOKED")
var ErrArtifactTooLarge = errors.New("EXPORT_ARTIFACT_TOO_LARGE")

// Access is rechecked both by the worker and download handler, including active account,
// membership, assigned branch, and the privilege level embedded in the artifact.
func ExportAccess(ctx context.Context, db database.Getter, biz, branch, user idkit.ID, kind string, cost, profit bool) error {
	var active bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND status='active')`, user).Scan(&active); err != nil {
		return err
	}
	if !active {
		return ErrExportPermission
	}
	repo := org.NewRepo(db)
	m, err := repo.MembershipFor(ctx, user, biz)
	if errors.Is(err, org.ErrNotMember) {
		return ErrExportPermission
	}
	if err != nil {
		return err
	}
	if m.Status != "active" {
		return ErrExportPermission
	}
	branches, err := repo.BranchIDsForMembership(ctx, m.ID)
	if err != nil {
		return err
	}
	tc := tenant.Context{Role: m.Role, BranchIDs: branches}
	perm := authz.PermReportSales
	if kind == "stock" {
		perm = authz.PermProductView
	}
	if !tc.HasBranch(branch) || authz.Check(m.Role, perm) == authz.Denied {
		return ErrExportPermission
	}
	if cost && authz.Check(m.Role, authz.PermProductViewCost) != authz.Allowed {
		return ErrExportPermission
	}
	if profit && authz.Check(m.Role, authz.PermReportProfit) != authz.Allowed {
		return ErrExportPermission
	}
	return nil
}

// CSV uses JSON field names/values from the same report DTOs, so numeric/null semantics
// cannot drift. This background format uses full RFC3339 timestamps, not localized text.
func exportCSV(w io.Writer, data any, hide map[string]bool) (int64, error) {
	value := reflect.ValueOf(data)
	if value.Kind() != reflect.Slice {
		return 0, errors.New("invalid export data")
	}
	typ := value.Type().Elem()
	fields := []string{}
	for i := 0; i < typ.NumField(); i++ {
		name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		if name != "" && name != "-" && !hide[name] {
			fields = append(fields, name)
		}
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(fields); err != nil {
		return 0, err
	}
	for i := 0; i < value.Len(); i++ {
		raw, err := json.Marshal(value.Index(i).Interface())
		if err != nil {
			return 0, err
		}
		var row map[string]json.RawMessage
		if err := json.Unmarshal(raw, &row); err != nil {
			return 0, err
		}
		cells := make([]string, len(fields))
		for j, key := range fields {
			raw := row[key]
			if len(raw) == 0 || string(raw) == "null" {
				continue
			}
			if raw[0] == '"' {
				if err := json.Unmarshal(raw, &cells[j]); err != nil {
					return 0, err
				}
				trimmed := strings.TrimLeft(cells[j], " \t\r\n")
				if (key == "reason" || key == "product_name" || key == "sku" || key == "base_unit" || key == "receipt_number") && trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
					cells[j] = "'" + cells[j]
				}
			} else {
				cells[j] = string(raw)
			}
		}
		if err := cw.Write(cells); err != nil {
			return 0, err
		}
	}
	cw.Flush()
	return int64(value.Len()), cw.Error()
}

type boundedBuffer struct{ bytes.Buffer }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	// shortcut: PostgreSQL artifacts limited to 50 MiB; use streamed object storage for larger files.
	if b.Len()+len(p) > 50<<20 {
		return 0, ErrArtifactTooLarge
	}
	return b.Buffer.Write(p)
}

// ProcessExport holds the claim and artifact in one transaction. SKIP LOCKED permits
// multiple workers; crash rolls back the claim, allowing retry without a stuck lease.
func ProcessExport(ctx context.Context, pool *database.Pool) (bool, error) {
	processed := false
	err := pool.InTx(ctx, func(ctx context.Context, tx database.Tx) error {
		processed = false
		if _, err := tx.Exec(ctx, `DELETE FROM public_receipt_limits WHERE window_start < now()-interval '1 day'`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE report_exports SET artifact=NULL,status='expired' WHERE expires_at<=now() AND status<>'expired'`); err != nil {
			return err
		}
		var id, biz, branch, user idkit.ID
		var kind string
		var raw []byte
		var cost, profit bool
		err := tx.QueryRow(ctx, `SELECT id,business_id,branch_id,requested_by,report_kind,parameters,cost_visible,profit_visible FROM report_exports WHERE status='pending' AND expires_at>now() ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &biz, &branch, &user, &kind, &raw, &cost, &profit)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		var p ExportParameters
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		p.Filter.BusinessID = biz
		p.Filter.BranchID = branch
		p.Filter.Export = true
		err = ExportAccess(ctx, tx, biz, branch, user, kind, cost, profit)
		if err != nil && !errors.Is(err, ErrExportPermission) {
			return err
		}
		var data any
		if err == nil {
			switch kind {
			case "payments":
				data, err = Payments(ctx, tx, p.Filter)
			case "sales":
				data, err = Sales(ctx, tx, p.Filter)
			case "corrections":
				data, err = Corrections(ctx, tx, p.Filter)
			case "stock":
				data, err = Stock(ctx, tx, p.Filter, cost, p.StockFilter)
			case "products":
				var rows []ProductRow
				rows, err = Products(ctx, tx, p.Filter, profit)
				data = rows
				if err == nil && p.Group == "category" {
					data, err = Categories(rows)
				}
			default:
				return fmt.Errorf("unknown export kind %q", kind)
			}
		}
		hide := map[string]bool{}
		if !cost {
			hide["average_cost_cents"] = true
			hide["estimated_value_cents"] = true
		}
		if !profit {
			for _, key := range []string{"sold_cost_cents", "restock_cost_cents", "estimated_gross_profit_cents"} {
				hide[key] = true
			}
		}
		var buf boundedBuffer
		var n int64
		if err == nil {
			n, err = exportCSV(&buf, data, hide)
		}
		if err != nil {
			if !errors.Is(err, ErrExportPermission) && !errors.Is(err, ErrArtifactTooLarge) {
				return err
			}
			if _, e := tx.Exec(ctx, `UPDATE report_exports SET status='failed',error_code=$2,completed_at=now() WHERE id=$1`, id, err.Error()); e != nil {
				return e
			}
		} else {
			if _, err := tx.Exec(ctx, `UPDATE report_exports SET status='completed',artifact=$2,row_count=$3,completed_at=now() WHERE id=$1`, id, buf.Bytes(), n); err != nil {
				return err
			}
			if err := audit.Write(ctx, tx, audit.Entry{BusinessID: biz, BranchID: &branch, ActorUserID: user, Action: "report.exported", EntityType: "background_export", EntityID: id.String(), After: map[string]any{"kind": kind, "rows": n}}); err != nil {
				return err
			}
		}
		processed = true
		return nil
	})
	return processed, err
}
