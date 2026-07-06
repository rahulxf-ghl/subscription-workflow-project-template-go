// @@@SNIPSTART subscription-go-querybillinginfo-query
package main

import (
	"context"
	"log"
	"strconv"

	"go.temporal.io/sdk/client"
)

// WALKTHROUGH — reads live state with a [#4] QUERY (no DB).
//   QueryWorkflow(ctx, workflowID, runID, queryName) calls a read-only handler
//   registered inside the workflow and returns its current in-memory value here it
//   reads "billingperiodnumber" and "billingperiodchargeamount". In dev-commerce-engine
//   this becomes a Get/Query API method. Queries never mutate state.
func main() {
	// The client is a heavyweight object that should be created once per process.
	c, err := client.NewClient(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	// Query all Workflow Executions to get current billing information
	for i := 0; i < 5; i++ {
		bpnresp, err := c.QueryWorkflow(context.Background(),
			"SubscriptionsWorkflowId-"+strconv.Itoa(i), "", "billingperiodnumber")
		if err != nil {
			log.Fatalln("Unable to query workflow", err)
		}
		var result interface{}
		if err := bpnresp.Get(&result); err != nil {
			log.Fatalln("Unable to decode query result", err)
		}

		bpcresp, err := c.QueryWorkflow(context.Background(),
			"SubscriptionsWorkflowId-"+strconv.Itoa(i), "", "billingperiodchargeamount")
		if err != nil {
			log.Fatalln("Unable to query workflow", err)
		}
		var result2 interface{}
		if err := bpcresp.Get(&result2); err != nil {
			log.Fatalln("Unable to decode query result", err)
		}

		log.Println("Workflow:", "Id", "SubscriptionsWorkflowId-"+strconv.Itoa(i))
		log.Println("Billing Results", "Billing Period", result)
		log.Println("Billing Results", "Billing Period Charge", result2)
	}
}
// @@@SNIPEND
