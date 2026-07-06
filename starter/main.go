// @@@SNIPSTART subscription-go-workflow-execution-starter
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"subscription-workfow"
	"time"

	"go.temporal.io/sdk/client"
)

// WALKTHROUGH — the STARTER is just a client that starts a workflow. It is a standalone
//   binary here ONLY because this is a demo. In dev-commerce-engine there is no starter
//   binary: the ExecuteWorkflow call lives INSIDE a service method (like fulfilment's
//   IngestRequest -> executor.StartRequest), triggered by the fulfilment-request manifest.
//   Likewise cancelsubscription/ and querybillinginfo/ become Cancel/Query API methods,
//   not separate binaries.
func main() {
	// The client is a heavyweight object that should be created once per process.
	c, err := client.NewClient(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	// Config via env. Defaults are the original demo (5 customers, 10s periods).
	// To watch ONE workflow slowly, run:
	//   COUNT=1 TRIAL_SECONDS=15 BILLING_SECONDS=20 MAX_PERIODS=4 go run ./starter
	count := envInt("COUNT", 5)
	sub := subscription.Subscription{
		TrialPeriod:         time.Duration(envInt("TRIAL_SECONDS", 10)) * time.Second,
		BillingPeriod:       time.Duration(envInt("BILLING_SECONDS", 10)) * time.Second,
		MaxBillingPeriods:   envInt("MAX_PERIODS", 24),
		BillingPeriodCharge: 120,
	}

	// Plain-English banner so the run narrates itself (handy for recording).
	fmt.Println("──────────────────────────────────────────────────────────────")
	fmt.Printf("Starting %d subscription(s). Each one will:\n", count)
	fmt.Printf("  1. send a welcome email\n")
	fmt.Printf("  2. run a %s free trial (cancellable, no charge)\n", sub.TrialPeriod)
	fmt.Printf("  3. then charge $%d every %s, up to %d times (this is the recurring billing)\n",
		sub.BillingPeriodCharge, sub.BillingPeriod, sub.MaxBillingPeriods)
	fmt.Printf("  4. a failed charge is retried automatically (dunning); cancel/amount-change come in as signals\n")
	fmt.Println("Watch it live at  http://localhost:8233")
	fmt.Println("──────────────────────────────────────────────────────────────")

	// create one Workflow Execution per customer
	for i := 0; i < count; i++ {
		cust := subscription.Customer{
			FirstName:    "First Name" + strconv.Itoa(i),
			LastName:     "Last Name" + strconv.Itoa(i),
			Email:        "someemail" + strconv.Itoa(i),
			Subscription: sub,
			Id:           "Id-" + strconv.Itoa(i),
		}

		workflowOptions := client.StartWorkflowOptions{
			ID:                 "SubscriptionsWorkflow" + cust.Id,
			TaskQueue:          "SubscriptionsTaskQueueGo",
			WorkflowRunTimeout: time.Minute * 10,
		}

		we, err := c.ExecuteWorkflow(context.Background(), workflowOptions, subscription.SubscriptionWorkflow, cust)
		if err != nil {
			log.Fatalln("Unable to execute workflow", err)
		}

		log.Println("Started workflow", "WorkflowID", we.GetID(), "RunID", we.GetRunID())

	}
}

// envInt reads an int from an env var, falling back to def if unset/invalid.
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
// @@@SNIPEND
