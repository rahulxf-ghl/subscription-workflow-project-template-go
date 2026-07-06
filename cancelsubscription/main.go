// @@@SNIPSTART subscription-go-cancel-subscription-signal
package main

import (
	"context"
	"log"
	"strconv"

	"go.temporal.io/sdk/client"
)

// WALKTHROUGH — sends the [#3] "cancelsubscription" SIGNAL into running workflows.
//   SignalWorkflow(ctx, workflowID, runID, signalName, payload) delivers a message
//   into a specific running workflow; the handler in workflow.go applies it between
//   steps (no race with the billing loop). In dev-commerce-engine this is a Cancel
//   API method, not a binary. Note it loops over the same 5 workflow IDs the starter
//   created ("SubscriptionsWorkflowId-0".."-4").
func main() {
	// The client is a heavyweight object that should be created once per process.
	c, err := client.NewClient(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	// Signal all Workflow Executions and cancel the subscription
	for i := 0; i < 5; i++ {
		err = c.SignalWorkflow(context.Background(),
			"SubscriptionsWorkflowId-"+strconv.Itoa(i), "", "cancelsubscription", true)
		if err != nil {
			log.Fatalln("Unable to signal workflow", err)
		}
	}
}
// @@@SNIPEND
