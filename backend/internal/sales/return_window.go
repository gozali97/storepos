package sales

import (
	"errors"
	"time"
)

var ErrReturnWindow = errors.New("transaksi melewati batas hari retur bisnis")

// Compare calendar dates, not elapsed 24-hour durations. AddDate also handles DST.
func WithinReturnWindow(sold, now time.Time, location *time.Location, days int) bool {
	if days == 0 {
		return true
	}
	sold = sold.In(location)
	now = now.In(location)
	start := time.Date(sold.Year(), sold.Month(), sold.Day(), 0, 0, 0, 0, location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	return !today.Before(start) && !today.After(start.AddDate(0, 0, days))
}
