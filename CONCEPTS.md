# Temporal Concepts — the simple version

A plain-English primer. Read this once and the code + the UI will make sense. Every concept
is tied to this subscription example.

## The one big idea: durable execution
Normally, if your server restarts in the middle of a long job, you lose your place — you need
a database flag and custom "where was I?" logic to recover. **Temporal removes that.** You
write a normal function; Temporal records every step it takes. If the process crashes or you
deploy, Temporal re-runs the function from its recorded history and it lands on the exact same
line, with the same variables. Your code just... survives.

That's why a subscription can "sleep for 30 days," get through a deploy, and wake up to charge.

---

## The pieces

### Workflow
The long-running function that describes the whole process. Here it's `SubscriptionWorkflow`:
welcome → trial → charge every period → end. **One workflow instance = one subscription**, for
its entire life (which can be months or years).

Rule: a workflow must be **deterministic** — re-running it must make the same decisions. So
inside a workflow you never do I/O, read the clock, use randomness, or call the network
directly. (Those go in activities.) This is what makes replay-after-crash possible.

### Activity
The **only** place you're allowed to touch the outside world: charge a card, send an email,
write to a DB. Here they're in `activities.go` (`ChargeCustomerForBillingPeriod`, the emails).
Temporal is what **retries** an activity if it fails.

Think of it as: the **workflow decides what to do**, the **activities actually do it**.

### Worker
The process that runs your workflow + activity code. It connects to Temporal and waits for
work. Here it's `worker/main.go` (you run it with `make worker`). No worker running = nothing
happens, even if you start a workflow. In production this is a long-lived service.

### Task Queue
The named mailbox workers listen on (here: `SubscriptionsTaskQueueGo`). The starter puts work
on the queue; the worker pulls from it. **They must use the same queue name** or they never meet.

### Client / Starter
Code *outside* the workflow that kicks it off or talks to it. `starter/main.go` starts
subscriptions; `cancelsubscription/` and `updatechargeamount/` send signals; `querybillinginfo/`
reads state. In a real service these become API endpoints, not separate programs.

### Workflow ID
Every workflow run has an ID. Here it's the customer ID (`SubscriptionsWorkflow` + `Id-0`).
Because the ID is the business entity, Temporal guarantees **at most one live run per customer**
— you can't accidentally start two billing loops for the same person.

---

## The four superpowers (what you'd otherwise hand-build)

### 1. Durable Timer — "wait for later"
`workflow.AwaitWithTimeout(ctx, duration, ...)` (and `NewTimer`) makes the workflow **sleep**
for seconds or months and wake itself. It costs almost nothing while asleep and survives
restarts. Here it's the free trial and the wait between charges. *(Without Temporal: a
scheduled task + a callback endpoint + a "next run" DB column + a cron to catch misses.)*

### 2. Retry Policy — "try again on failure" (dunning)
A few lines of config on an activity: how many attempts, how fast the backoff grows. If the
charge fails, Temporal retries it automatically on that schedule. Here: 5s, then 10s, max 3
tries. *(Without Temporal: an attempt counter, backoff math, a re-enqueue, a dead-letter queue.)*

### 3. Signal — "send a message into a running workflow"
An outside caller pushes data into a *running* workflow, applied between steps (so it can't
race the billing loop). Here: `cancelsubscription` (stop it) and `billingperiodcharge` (change
the amount). *(Without Temporal: a DB write plus locking so it doesn't collide with in-flight work.)*

### 4. Query — "read a running workflow's live state"
A read-only peek at the workflow's current in-memory values, no database. Here:
`billingperiodnumber`, `billingperiodchargeamount`. *(Without Temporal: a query endpoint over
the DB, and hoping the row reflects the truly current state.)*

---

## Two mechanics worth knowing

### Event History & Replay
Temporal records every step (timer started, activity scheduled, activity completed, signal
received) as an **event history**. That history *is* the Timeline you see in the Web UI, and
it's a free audit log. On a crash/deploy, Temporal replays the history to rebuild the
workflow's exact state — that's the durability. (And that's why workflows must be
deterministic: replay must produce the same steps.)

### Continue-As-New (not in this template — but important)
A workflow's history can't grow forever. For something that bills monthly for years, you
periodically call `workflow.NewContinueAsNewError(...)` to start a fresh run with the same ID
and a clean history, carrying the state forward. It's the standard pattern for long-lived
workflows. This template caps at `MaxBillingPeriods` instead, so it doesn't need it yet.

---

## 30-second glossary
- **Durable execution** — code that survives crashes/deploys by replaying recorded history.
- **Workflow** — the long-running, deterministic function (one per subscription).
- **Activity** — a single side-effecting step (charge, email); the retryable unit.
- **Worker** — the process that runs workflow + activity code.
- **Task Queue** — the named queue connecting starter and worker.
- **Signal** — a message *into* a running workflow (cancel, change amount).
- **Query** — a read of a running workflow's live state.
- **Timer** — a durable sleep.
- **Retry Policy** — declarative auto-retry for an activity (dunning).
- **Workflow ID** — the business ID; guarantees one live run per entity.
- **Event History** — the recorded list of steps; the source of truth and the audit log.
- **Continue-As-New** — restart a long-lived workflow with fresh history.
