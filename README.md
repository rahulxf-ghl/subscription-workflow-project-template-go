# Temporal Subscription Workflow — Go

A subscription billing engine on Temporal: free trial, recurring charges, dunning
(auto-retry on a failed charge), cancel, and change-amount — in ~150 lines.

**New to Temporal?** Read [CONCEPTS.md](CONCEPTS.md) first (plain-English primer), then come
back here to run it. For a deep, annotated walkthrough see [GUIDE.md](GUIDE.md); for a
recording script see [VIDEO_SCRIPT.md](VIDEO_SCRIPT.md).

---

## 1. Prerequisites

- **Go** 1.18+
- **Temporal CLI** (gives you a local server + Web UI, no Docker needed):
  ```bash
  brew install temporal        # macOS
  # others: https://docs.temporal.io/cli#install
  ```

> The upstream template used `docker-compose`. You don't need it — the Temporal CLI ships
> a full local server (`temporal server start-dev`), which is faster to start and stop.

## 2. Quick start (3 terminals)

```bash
make deps              # download Go modules (once)

# Terminal 1 — local Temporal server + Web UI at http://localhost:8233
make temporal

# Terminal 2 — the worker: the process that runs the workflow + activity code. Leave it running.
make worker

# Terminal 3 — start ONE subscription you can actually watch (5s trial, charge every 20s, x4)
make one
```

Open **http://localhost:8233**, click the workflow, and watch it live.
Run `make reset` any time to clear old runs.

## 3. Use cases (what each command does)

Every command is a `make` target. Pattern: **pick a worker in terminal 1, then run a
command in terminal 2.** To switch worker behavior, `Ctrl-C` the worker and start a different one.

### a) Normal billing
```bash
make worker      # charges always succeed
make one
```
**What happens:** welcome email → 5s free trial → a $120 charge every 20s, up to 4 times → a
"subscription over" email → the workflow Completes. This is the happy path.

### b) Read live state (Query)
```bash
make query-one           # single workflow
# or for the 5-customer demo:  go run ./querybillinginfo
```
**What it does:** asks the *running* workflow which billing period it's on — read straight
from the workflow's memory, no database. Run it a few times to watch the number climb.

### c) Change the charge amount (Signal)
```bash
make signal-amount       # sets the running subscription's charge to 300
# or for all 5:  go run ./updatechargeamount
```
**What it does:** sends a message into the running workflow that updates the billing amount
mid-cycle. No restart, no DB write racing the billing loop.

### d) Cancel (Signal)
```bash
make signal-cancel
```
**What it does:** sends a cancel into the running subscription. It stops at the current step,
sends the cancellation email, and Completes early. (Send it during the first 5s to cancel in
the trial; send it later to cancel during active billing.)

### e) Dunning that recovers ⭐ (a failed charge that retries and succeeds)
```bash
make worker-dunning      # charge fails attempts 1 & 2, succeeds on attempt 3
make one
```
**What happens:** each charge fails twice, and Temporal retries it automatically on the
schedule (5s, then 10s) until it goes through. In the UI the charge shows a **red→green** bar
with a `↻3` badge. **You wrote no retry code** — it's the `RetryPolicy` in `workflow.go`.

### f) Dunning that gives up (a charge that never succeeds)
```bash
make worker-dunning-fail # charge always fails
make one
```
**What happens:** the charge is tried 3 times, all fail, Temporal stops with
`MAXIMUM_ATTEMPTS_REACHED`, and the **workflow fails** (red). The worker keeps running — only
that one subscription failed.

### g) Crash / deploy survival (the killer demo)
```bash
make worker
make one
# after the first charge, Ctrl-C the worker; wait ~15s; then:
make worker
```
**What happens:** the subscription **resumes exactly where it was** — same billing period, same
pending timer — even though the process died. You wrote no resume logic; Temporal replays the
workflow from its recorded history.

## 4. How to read the Temporal Web UI (http://localhost:8233)

Click a workflow to open it. Useful tabs:
- **Timeline** — the visual ladder of the run (see the key below).
- **Event History** — every step as a list (timers, activities, signals). This is the free audit log.
- **Queries** — run a query (e.g. `billingperiodnumber`) right from the UI.
- **Pending Activities** — anything currently running or waiting to retry.

**Timeline color key:**
| What you see | Meaning |
| --- | --- |
| 🟩 full-width bar on top | the whole workflow — **green = Completed**, red = Failed |
| 🟧 orange bar `(Xs)` | the workflow **sleeping** on a durable timer (trial or billing wait) |
| 🟩 green diamond `ChargeCustomerForBillingPeriod` | a charge that succeeded |
| `↻3` **red→green** bar | a charge that failed then **recovered** (dunning worked) |
| `↻3` **solid red** bar | all retries failed (dunning **gave up**) |
| 🟪 `cancelsubscription` marker | a signal arrived |

Numbers like `165 (10s)` are event sequence IDs + the timer duration — not money.

## 5. Where the code lives (quick map)

| File | Role |
| --- | --- |
| `workflow.go` | the subscription lifecycle (trial → billing loop → cancel/expiry). Has `[#1]`–`[#6]` markers. |
| `activities.go` | the side-effecting steps (charge, emails) — the only place I/O is allowed |
| `worker/main.go` | the worker process (runs workflow + activities) |
| `starter/`, `cancelsubscription/`, `updatechargeamount/`, `querybillinginfo/` | small clients that start/signal/query |

## 6. All make targets
```bash
make help    # lists everything with descriptions
```
