# Subscription Workflow — dev commands
# Requires: Go, and the Temporal CLI (https://docs.temporal.io/cli) for `make temporal`.

.PHONY: help deps build temporal worker start cancel query ui tidy \
        one reset worker-dunning worker-dunning-fail signal-cancel signal-amount query-one

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

deps: ## Download Go module dependencies
	go mod download

tidy: ## Tidy go.mod/go.sum
	go mod tidy

build: ## Compile everything (fast sanity check)
	go build ./...

temporal: ## Start a local Temporal dev server + Web UI (http://localhost:8233)
	temporal server start-dev

worker: ## Run the worker normally (charges always succeed). Keep this running.
	go run ./worker

worker-dunning: ## WORKER for the dunning demo: charge fails attempts 1&2, succeeds on 3
	CHARGE_FAIL_UNTIL_ATTEMPT=3 go run ./worker

worker-dunning-fail: ## WORKER where charge ALWAYS fails: dunning gives up, workflow fails
	CHARGE_FAIL_ALWAYS=1 go run ./worker

# --- run ONE subscription so a single use case is easy to watch/record ---
# Flow: pick a worker (worker / worker-dunning / worker-dunning-fail) in terminal 1,
#       then run `make one` in terminal 2 and watch http://localhost:8233

one: ## Start ONE slow subscription (5s trial, charge every 20s, 4 charges)
	COUNT=1 TRIAL_SECONDS=5 BILLING_SECONDS=20 MAX_PERIODS=4 go run ./starter

reset: ## Terminate leftover demo workflows so the UI is clean
	@for id in 0 1 2 3 4; do temporal workflow terminate --workflow-id SubscriptionsWorkflowId-$$id --reason cleanup 2>/dev/null || true; done
	@echo "cleared."

signal-cancel: ## Cancel the single workflow (SubscriptionsWorkflowId-0)
	temporal workflow signal --workflow-id SubscriptionsWorkflowId-0 --name cancelsubscription --input true

signal-amount: ## Change the single workflow's charge amount to 300
	temporal workflow signal --workflow-id SubscriptionsWorkflowId-0 --name billingperiodcharge --input 300

query-one: ## Query the single workflow's current billing period
	temporal workflow query --workflow-id SubscriptionsWorkflowId-0 --type billingperiodnumber

start: ## Start 5 subscription workflows (the "create subscription" entry point)
	go run ./starter

cancel: ## Send the cancel signal to all 5 workflows
	go run ./cancelsubscription

query: ## Query live billing state from all 5 workflows
	go run ./querybillinginfo

ui: ## Open the Temporal Web UI
	open http://localhost:8233
