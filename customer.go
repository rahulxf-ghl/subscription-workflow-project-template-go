// @@@SNIPSTART subscription-go-customer-struct
package subscription

// Customer is the workflow input. Id is the important field: the starter uses it to
// build the Workflow ID ("SubscriptionsWorkflow"+Id), which makes each customer map
// to exactly one workflow — the "one workflow per real-world thing" rule.
type Customer struct {
	FirstName    string
	LastName     string
	Id           string // used as the workflow-id suffix -> one run per customer
	Email        string
	Subscription Subscription
}
// @@@SNIPEND
