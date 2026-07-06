# Subscription Workflow — Guide (run it + understand every file)

An annotated walkthrough of the Temporal subscription template, plus a side-by-side of
what each Temporal feature would cost to hand-build on **Cloud Run + Cloud Tasks**.
Read this alongside the `[#N]` markers in `workflow.go`.

---

## 1. Prerequisites

- **Go** (1.18+).
- **Temporal CLI** for a local server — install: https://docs.temporal.io/cli
  (`brew install temporal` on macOS).

## 2. Run it (4 terminals, or use the Makefile)

```bash
make deps          # download Go deps

# Terminal 1 — local Temporal server + Web UI at http://localhost:8233
make temporal

# Terminal 2 — the worker (hosts workflow + activities). Leave running.
make worker

# Terminal 3 — start 5 subscription workflows (the "create subscription" call)
make start
```

Now open **http://localhost:8233** (`make ui`) — you'll see 5 running workflows.
While they run you can:

```bash
make query         # read live billing state (Query, no DB)
make cancel        # send the cancel signal to all 5
```

Timing: the starter uses a 10s trial + 10s billing period + 24 periods, so a run lasts
~4 minutes. In production those become days/weeks — the code is identical, only the
durations change.

> No Makefile? The raw commands are `temporal server start-dev`, `go run ./worker`,
> `go run ./starter`, `go run ./querybillinginfo`, `go run ./cancelsubscription`.

## 3. What to watch in the Web UI

- **Event History** of a workflow — every step (timer started, activity scheduled,
  activity completed, signal received) is recorded. That history is point **#6** below.
- Send `make cancel`, then refresh — you'll see a `WorkflowExecutionSignaled` event and
  the run finish early. That's point **#3**.

---

## 4. The point: Cloud Run + Cloud Tasks vs Temporal

Each row is marked in `workflow.go` with a `[#N]` comment.

| # | Need | Cloud Run + Cloud Tasks (by hand) | Temporal (built-in) |
| --- | --- | --- | --- |
| 1 | Wait for next billing date | delayed Cloud Task + callback endpoint + `next_billing_date` column + reconciler cron for missed tasks | `AwaitWithTimeout(BillingPeriod)` — one line |
| 2 | Retry a failed charge (dunning) | re-enqueue task, attempt counter in DB, backoff math, dead-letter handling | activity `RetryPolicy` (60s / double / max-3), declared once |
| 3 | Change amount / cancel mid-cycle | DB update + locking so it doesn't race the in-flight billing task | a **signal** (applied between steps, no race) |
| 4 | Read current state | query the DB row | a **Query** to the workflow's own memory |
| 5 | Survive a crash / deploy | design idempotency + resume logic yourself | automatic replay from history |
| 6 | Audit every attempt | log/store each attempt to your own table | Temporal history, free |

One line: **the Cloud Tasks version is a table + a queue + a callback endpoint + a
dunning counter + a reconciler cron + hand-rolled idempotency. The Temporal version is
one function plus its activities.**

---

## 5. File-by-file

| File | What it is | In dev-commerce-engine it becomes |
| --- | --- | --- |
| `customer.go`, `subscription.go` | workflow input structs | the subscription entity/proto |
| `workflow.go` | the whole lifecycle: trial → billing loop → cancel/expiry, with signals + queries | `SubscriptionWorkflow`, owned by the subscription team |
| `activities.go` | the side-effecting steps (charge, emails) — **the only place I/O is allowed** | real calls to PPC + notifications |
| `worker/main.go` | the process that runs workflow + activity code | `cmd/subscription-worker`, using the shared `common/temporal` wrapper |
| `starter/main.go` | client that starts a workflow per customer | **not a binary** — an `ExecuteWorkflow` call inside a service method, triggered by the fulfilment-request manifest |
| `cancelsubscription/main.go` | client that sends the cancel signal | a `Cancel` API method |
| `querybillinginfo/main.go` | client that queries state | a `Get`/`Query` API method |

**Mental model:** in the template these are separate `main.go` binaries because it's a
demo. In a real service, start/cancel/query collapse into API methods on one service
(exactly like the fulfilment platform's `IngestRequest` / `Cancel` / `Query`).

## 6. Ownership (for the Commerce team)

- **Platform (Rahul):** the shared `common/temporal` wrapper, the port/adapter pattern,
  TLS/config, versioning convention, and these docs. The rails every module runs on.
- **Subscription team (Aditya/Parth):** the `SubscriptionWorkflow`, its activities, the
  subscription entities, and the `subscription-worker` binary — built on the platform.

---

## 7. What this template does NOT cover yet (add for production)

The template proves the *shape*. A real subscription that bills for years still needs:

1. **Dunning retry** — ✅ added here as a `RetryPolicy` in `workflow.go` (the upstream
   template had none).
2. **Continue-As-New** — the billing loop caps at `MaxBillingPeriods` in one history. A
   subscription billing monthly for years must call `workflow.NewContinueAsNewError` each
   cycle to keep history small. **Not in this template — biggest gap.**
3. **Pause / resume + plan-change (proration)** signals — only cancel + change-amount exist.
4. **Real activities** — charge via PPC, emit Pub/Sub events, persist to the operator store.
5. **Idempotency keys** on the charge — so a retry/replay never double-charges (principle #3).
6. **Creation from the fulfilment-request manifest** — subscription is created *by* a
   fulfilment request, not a standalone starter.
7. **Hardening** — replace the `log.Fatalln` calls in the workflow (they'd crash the
   worker) with proper error returns, and route signals through one main Selector loop.
