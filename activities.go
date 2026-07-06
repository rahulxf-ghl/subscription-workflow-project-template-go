// @@@SNIPSTART subscription-go-activities
package subscription

import (
	"context"

	"go.temporal.io/sdk/activity"
)

// ============================================================================
// ACTIVITIES = the ONLY place the workflow is allowed to touch the outside world.
//
// Rule (design principle #2): workflow.go must be deterministic — no network, no
// DB, no clock, no randomness. Anything that talks to the real world (charge a
// card, send an email, write a row) lives here, in an Activity. Temporal is what
// retries an Activity (see the RetryPolicy in workflow.go [#2]).
//
// These implementations are STUBS — they just log. In dev-commerce-engine each
// becomes a real call:
//   ChargeCustomerForBillingPeriod -> call PPC (Payment Provider Core) to charge,
//     carrying an idempotency key so a retry never double-charges (principle #3).
//   Send*Email                     -> call the notification service.
// The `Activities` struct is a receiver so real deps (a PPC client, a repo) can be
// injected as fields later; the worker registers it with RegisterActivity(&Activities{}).
// ============================================================================

type Activities struct {
	// Real version: PPC client, notification client, repo, etc. injected here.
}

func (a *Activities) SendWelcomeEmail(ctx context.Context, customer Customer) (string, error) {
	activity.GetLogger(ctx).Info("sending welcome email to customer", customer.Id)
	return "Sending welcome email completed for " + customer.Id, nil
}

func (a *Activities) SendCancellationEmailDuringTrialPeriod(ctx context.Context, customer Customer) (string, error) {
	activity.GetLogger(ctx).Info("sending cancellation email during trial period to: ", customer.Email)
	return "Sending cancellation email during trial period completed for " + customer.Id, nil
}

// ChargeCustomerForBillingPeriod is the money step. If it returns an error, the
// RetryPolicy declared in workflow.go retries it automatically (the "dunning" loop).
// Real version: call PPC with an idempotency key derived from (customerId, periodNum)
// so retries and replays never charge twice.
func (a *Activities) ChargeCustomerForBillingPeriod(ctx context.Context, customer Customer) (string, error) {
	activity.GetLogger(ctx).Info("charging customer for billing period.")
	return "Charging for billing period completed for: " + customer.Id, nil
}

func (a *Activities) SendCancellationEmailDuringActiveSubscription(ctx context.Context, customer Customer) (string, error) {
	activity.GetLogger(ctx).Info("sending cancellation email during active subscription to: ", customer.Id)
	return "Sending cancellation email during active subscription completed for: " + customer.Id, nil
}

func (a *Activities) SendSubscriptionOverEmail(ctx context.Context, customer Customer) (string, error) {
	activity.GetLogger(ctx).Info("sending subscription over email to: ", customer.Id)
	return "Sending subscription over email completed for: " + customer.Id, nil
}
// @@@SNIPEND
