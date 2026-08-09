package domain

import (
	"strings"
	"time"
	_ "time/tzdata"
)

var reversalReasons = map[string]struct{}{
	"OPERATOR_ERROR":     {},
	"CUSTOMER_RETURN":    {},
	"CUSTOMER_COMPLAINT": {},
	"TAX_BASE_REDUCTION": {},
}

// reversalAllowed applies Art. 31(4) in the Europe/Sofia civil calendar.
// An operator-error reversal is allowed through the whole seventh day of the
// month following the error. Other documented reasons are event-bound rather
// than age-bound; the API invocation is the system timestamp of that event.
func reversalAllowed(reason string, original, now time.Time) bool {
	reason = strings.TrimSpace(reason)
	if _, ok := reversalReasons[reason]; !ok || original.IsZero() || now.Before(original) {
		return false
	}
	if reason != "OPERATOR_ERROR" {
		return true
	}
	location, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		return false
	}
	originalLocal := original.In(location)
	deadlineExclusive := time.Date(originalLocal.Year(), originalLocal.Month()+1, 8, 0, 0, 0, 0, location)
	return now.In(location).Before(deadlineExclusive)
}
