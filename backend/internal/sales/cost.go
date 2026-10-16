package sales

import (
	"errors"
	"math/big"
)

// SnapshotCost uses base milli-units and half-up to sen without overflowing price*qty.
func SnapshotCost(price, qty int64) (int64, error) {
	if price < 0 || qty < 0 {
		return 0, errors.New("HPP/kuantitas tidak valid")
	}
	n := new(big.Int).Mul(big.NewInt(price), big.NewInt(qty))
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, big.NewInt(1000), r)
	if r.Int64() >= 500 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() {
		return 0, errors.New("HPP melampaui kapasitas integer")
	}
	return q.Int64(), nil
}

// CostSlice rounds to sen (not rupiah); final return receives exact residual cost.
func CostSlice(cost, sold, returned, prior, qty int64) (int64, error) {
	if cost < 0 || sold <= 0 || returned < 0 || qty <= 0 || returned > sold || qty > sold-returned || prior < 0 || prior > cost {
		return 0, ErrReturnQuantity
	}
	if qty == sold-returned {
		return cost - prior, nil
	}
	n := new(big.Int).Mul(big.NewInt(cost), big.NewInt(qty))
	den := big.NewInt(sold)
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, den, r)
	if new(big.Int).Lsh(r, 1).Cmp(den) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() {
		return 0, ErrReturnQuantity
	}
	amount := q.Int64()
	if amount > cost-prior {
		amount = cost - prior
	}
	return amount, nil
}
