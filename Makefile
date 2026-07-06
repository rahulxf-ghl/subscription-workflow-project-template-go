# Subscription Workflow — dev commands
# Requires: Go, and the Temporal CLI (https://docs.temporal.io/cli) for `make temporal`.

.PHONY: help deps build temporal worker start cancel query ui tidy

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

worker: ## Run the worker (hosts the workflow + activities). Keep this running.
	go run ./worker

start: ## Start 5 subscription workflows (the "create subscription" entry point)
	go run ./starter

cancel: ## Send the cancel signal to all 5 workflows
	go run ./cancelsubscription

query: ## Query live billing state from all 5 workflows
	go run ./querybillinginfo

ui: ## Open the Temporal Web UI
	open http://localhost:8233
