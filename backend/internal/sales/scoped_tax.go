package sales

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/storepos/storepos/internal/platform/database"
	"github.com/storepos/storepos/internal/platform/idkit"
	"github.com/storepos/storepos/internal/pricing"
)

// Product override wins over category, then legacy business default. Zero-rate rule is explicit exemption.
func EffectiveTax(ctx context.Context, db database.Getter, biz, product idkit.ID, fallback *pricing.TaxRule) (*pricing.TaxRule, error) {
	var name, mode string
	var rate int64
	err := db.QueryRow(ctx, `SELECT r.name,r.rate_bps,r.mode FROM products p LEFT JOIN categories c ON c.business_id=p.business_id AND c.id=p.category_id JOIN scoped_tax_rules r ON r.business_id=p.business_id AND r.id=COALESCE(p.tax_rule_id,c.tax_rule_id) WHERE p.business_id=$1 AND p.id=$2 AND NOT r.archived`, biz, product).Scan(&name, &rate, &mode)
	if errors.Is(err, pgx.ErrNoRows) {
		return fallback, nil
	}
	if err != nil {
		return nil, err
	}
	rule := &pricing.TaxRule{Name: name, RateBps: rate, Mode: pricing.TaxExclusive}
	if mode == "inclusive" {
		rule.Mode = pricing.TaxInclusive
	}
	return rule, nil
}
