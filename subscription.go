// @@@SNIPSTART subscription-go-subscription-struct
package subscription

import "time"

// Subscription is the plan config passed into the workflow as input. Because it is
// the workflow's argument, Temporal records it in history — so on a replay after a
// crash the workflow sees the exact same values.
type Subscription struct {
	TrialPeriod         time.Duration // how long the free trial lasts ([#1] durable timer sleeps this long)
	BillingPeriod       time.Duration // gap between charges, e.g. ~30 days ([#1] durable timer)
	MaxBillingPeriods   int           // stop after this many charges (bounds the loop; see Continue-As-New note in GUIDE.md)
	BillingPeriodCharge int           // amount per charge; can be changed live via the [#3] "billingperiodcharge" signal
}
// @@@SNIPEND
