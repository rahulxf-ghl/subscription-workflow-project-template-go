// @@@SNIPSTART subscription-go-workflow-definition
package subscription

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ============================================================================
// WALKTHROUGH MAP — Cloud Run + Cloud Tasks (by hand)  vs  Temporal (built-in)
// Each numbered marker below shows where Temporal gives us something for free
// that we would otherwise hand-build on Cloud Tasks.
//
//   #1 Durable timer      "wait for next billing date"     -> see [#1] AwaitWithTimeout
//   #2 Retry / dunning    "retry a failed charge"          -> see [#2] RetryPolicy
//   #3 Signal             "change amount / cancel"         -> see [#3] signal channels
//   #4 Query              "read current state"             -> see [#4] query handlers
//   #5 Crash/deploy safe  "survive a restart"              -> see [#5] (automatic replay)
//   #6 Audit              "record every attempt"           -> see [#6] (Temporal history)
// ============================================================================

func SubscriptionWorkflow(ctx workflow.Context, customer Customer) (string, error) {
	// [#5] CRASH / DEPLOY SAFETY (built-in, no code):
	//   Cloud Run version: if the pod restarts mid-billing you must rebuild "where
	//     were we" from a DB flag + the queue, and design every step to be resumable.
	//   Temporal version : this function is re-executed from its recorded history on
	//     any restart/deploy and lands on the exact same line, same variables. The
	//     local vars below (billingPeriodNum, subscriptionCancelled) survive crashes
	//     for free — that is why they must stay deterministic (no clock/rand/IO here).
	workflowCustomer := customer
	subscriptionCancelled := false
	billingPeriodNum := 0
	actResult := ""

	QueryCustomerIdName := "customerid"
	QueryBillingPeriodNumberName := "billingperiodnumber"
	QueryBillingPeriodChargeAmountName := "billingperiodchargeamount"

	logger := workflow.GetLogger(ctx)

	// [#4] QUERY — "read current state without touching a DB":
	//   Cloud Run version: expose an endpoint that SELECTs the subscription row, and
	//     hope the row reflects the truly current in-flight state.
	//   Temporal version : register read-only handlers that return the workflow's own
	//     live memory. querybillinginfo/main.go calls these. Zero DB, always current.
	//
	// Define query handlers
	// Register query handler to return trip count
	err := workflow.SetQueryHandler(ctx, QueryCustomerIdName, func() (string, error) {
		return workflowCustomer.Id, nil
	})
	if err != nil {
		logger.Info("QueryCustomerIdName handler failed.", "Error", err)
		return "Error", err
	}

	err = workflow.SetQueryHandler(ctx, QueryBillingPeriodNumberName, func() (int, error) {
		return billingPeriodNum, nil
	})
	if err != nil {
		logger.Info("QueryBillingPeriodNumberName handler failed.", "Error", err)
		return "Error", err
	}

	err = workflow.SetQueryHandler(ctx, QueryBillingPeriodChargeAmountName, func() (int, error) {
		return workflowCustomer.Subscription.BillingPeriodCharge, nil
	})
	if err != nil {
		logger.Info("QueryBillingPeriodChargeAmountName handler failed.", "Error", err)
		return "Error", err
	}
	// end defining query handlers

	// [#3] SIGNAL — "change amount / cancel mid-cycle, without a race":
	//   Cloud Run version: UPDATE the DB row, then add locking so the change does not
	//     race the in-flight billing Cloud Task, which may have already read old state.
	//   Temporal version : an external caller sends a signal (see cancelsubscription/
	//     main.go). It is delivered into THIS running workflow and applied between
	//     steps, so it can never race the billing loop. No locks, no correlation table.
	//
	// Just grab the signal channels here. We CONSUME them inside the waitOrTimeout
	// helper (defined below) using a single Selector — the correct pattern.
	// (The original template registered AddReceive callbacks but then waited on
	// HasPending() without ever calling Select(), so a cancel signal was received
	// but never applied — the billing loop then charged every period anyway.)
	chargeCh := workflow.GetSignalChannel(ctx, "billingperiodcharge")
	cancelCh := workflow.GetSignalChannel(ctx, "cancelsubscription")

	// [#2] RETRY / DUNNING — "retry a failed charge on a schedule":
	//   Cloud Run version: on a failed charge, re-enqueue a Cloud Task, keep an attempt
	//     counter in the DB, compute the backoff yourself, and build dead-letter handling.
	//   Temporal version : declare the policy ONCE here. Every activity below (including
	//     ChargeCustomerForBillingPeriod) is retried automatically, crash-safe, on this
	//     schedule. This RetryPolicy is exactly the dunning rule (first retry 60s, double
	//     each time, max 3 attempts) — with no retry loop written anywhere.
	//   NOTE: the upstream template omits RetryPolicy (it only sets a timeout); this was
	//     added so point #2 is real, runnable code you can point at.
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 5,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 5, // short so retries are watchable in the demo; PROD dunning uses 60s
			BackoffCoefficient: 2.0,             // double the wait each attempt (5s, 10s, ...)
			MaximumAttempts:    3,               // give up after 3 tries
		},
	}

	ctx = workflow.WithActivityOptions(ctx, ao)
	logger.Info("Subscription workflow started for: " + customer.Id)

	// waitOrTimeout blocks up to d, but wakes EARLY and applies signals as they
	// arrive. It uses ONE Selector that waits on the timer OR the signal channels,
	// and actually consumes each signal. This is the fix for the cancel bug: a
	// cancel now sets subscriptionCancelled and breaks the wait immediately.
	waitOrTimeout := func(d time.Duration) {
		timerCtx, cancelTimer := workflow.WithCancel(ctx)
		defer cancelTimer()
		timer := workflow.NewTimer(timerCtx, d)
		timerFired := false
		for !timerFired && !subscriptionCancelled {
			sel := workflow.NewSelector(ctx)
			sel.AddFuture(timer, func(workflow.Future) { timerFired = true })
			sel.AddReceive(cancelCh, func(c workflow.ReceiveChannel, _ bool) {
				c.Receive(ctx, &subscriptionCancelled) // "cancelsubscription" signal
			})
			sel.AddReceive(chargeCh, func(c workflow.ReceiveChannel, _ bool) {
				var amount int
				c.Receive(ctx, &amount) // "billingperiodcharge" signal
				workflowCustomer.Subscription.BillingPeriodCharge = amount
			})
			sel.Select(ctx)
		}
	}

	var activities *Activities

	// Send welcome email to customer (best-effort: log and continue, do NOT crash).
	err = workflow.ExecuteActivity(ctx, activities.SendWelcomeEmail, workflowCustomer).Get(ctx, &actResult)
	if err != nil {
		logger.Error("failed sending welcome email", "Error", err)
	}

	// [#1] DURABLE TIMER — "wait for a future moment (here: the whole trial period)":
	//   Cloud Run version: schedule a Cloud Task with an ETA + a callback endpoint to
	//     fire when the trial ends, plus a DB column for the trial-end date, plus a
	//     reconciler cron in case the task never fires.
	//   Temporal version : the workflow simply sleeps for TrialPeriod (could be 30 days)
	//     and wakes itself. It costs ~nothing while asleep and survives deploys. The
	//     second arg lets it wake EARLY if the customer cancels during the trial.
	// Start the free trial period. User can still cancel subscription during this time.
	// waitOrTimeout wakes early if a cancel signal arrives during the trial.
	waitOrTimeout(workflowCustomer.Subscription.TrialPeriod)

	// If customer cancelled their subscription during trial period, send notification email
	if subscriptionCancelled == true {
		err = workflow.ExecuteActivity(ctx, activities.SendCancellationEmailDuringTrialPeriod, workflowCustomer).Get(ctx, &actResult)
		if err != nil {
			logger.Error("failed sending trial-cancellation email", "Error", err)
		}
		// We have completed subscription for this customer.
		// Finishing workflow execution
		return "Subscription finished for: " + workflowCustomer.Id, err
	}

	// Trial period is over, start billing until
	// we reach the max billing periods for the subscription
	// or sub has been cancelled
	for {
		if billingPeriodNum >= workflowCustomer.Subscription.MaxBillingPeriods {
			break
		}

		// [#2]+[#6] the charge runs with the RetryPolicy above (auto dunning), and
		// [#6] AUDIT (built-in, no code): every attempt of this activity — inputs,
		//   result, failures, retry count — is recorded in Temporal history automatically.
		//   Cloud Run version: you would log/store each attempt to your own audit table.
		// Charge customer for the billing period
		err = workflow.ExecuteActivity(ctx, activities.ChargeCustomerForBillingPeriod, workflowCustomer).Get(ctx, &actResult)
		if err != nil {
			// Dunning gave up (all retries exhausted). Return the error so the WORKFLOW
			// fails cleanly and shows as Failed in the UI. (The upstream template called
			// log.Fatalln here, which crashes the whole worker — wrong for a real system.)
			logger.Error("charge failed after all retries; failing subscription", "Error", err)
			return "Subscription failed for: " + workflowCustomer.Id, err
		}
		// [#1] DURABLE TIMER — "wait for the next billing date":
		//   Cloud Run version: a delayed Cloud Task + callback endpoint + next_billing_date
		//     DB column + a reconciler cron for tasks that never fire.
		//   Temporal version : sleep one BillingPeriod (e.g. ~30 days) right here, or wake
		//     early if a cancel signal is pending. This single line replaces all of the above.
		// Wait 1 billing period, OR wake early if a cancel/charge-change signal arrives.
		waitOrTimeout(workflowCustomer.Subscription.BillingPeriod)

		if subscriptionCancelled {
			err = workflow.ExecuteActivity(ctx, activities.SendCancellationEmailDuringActiveSubscription, workflowCustomer).Get(ctx, &actResult)
			if err != nil {
				logger.Error("failed sending active-cancellation email", "Error", err)
			}
			break
		}

		billingPeriodNum++
	}

	// if we get here the subscription period is over
	// notify the customer to buy a new subscription
	if !subscriptionCancelled {
		err = workflow.ExecuteActivity(ctx, activities.SendSubscriptionOverEmail, workflowCustomer).Get(ctx, &actResult)
		if err != nil {
			logger.Error("failed sending subscription-over email", "Error", err)
		}
	}

	return "Completed Subscription Workflow", err
}
// @@@SNIPEND
