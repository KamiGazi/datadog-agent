# Manual verification: capture-failure reasons in snapshot output

This recipe drives the eBPF capture path through each internal limit
that the data-item reason bits, drop-notification cause codes, and
synthetic `evaluationErrors` are designed to surface. Use it after
changes that touch the reason machinery to confirm the rendered
snapshot JSON reports the right cause instead of falling back to
`depth` / `unavailable`.

## Setup

1. Build the agent locally and place a config under `dev/dist/datadog.yaml`
   with a fake API key:
   ```
   api_key: 0000001
   ```
2. Run the agent under `sudo -E` so the eBPF programs can load (the
   stack machine and ring buffers require capabilities).
3. Tail the log uploader's output so emitted snapshots are visible.

## Scenarios

Each scenario is a probe configuration that deterministically forces
one capture-failure cause. Run the probe, trigger the function, and
inspect the emitted snapshot JSON. The "expected" column is the
`notCapturedReason` or `evaluationErrors` string the new
reason-propagation work should produce; the "regression check" column
is what today's pre-change code would have emitted (and what should
no longer appear).

Schema rule: `notCapturedReason` is reserved for values that have
**no captured bytes**. When a value is partially captured (a string
clamped to `MaxLength`, or a value clamped to the 8 KiB per-item
ceiling), the existing `size` + `truncated: true` pair already tells
the consumer the original length and the captured prefix; no
`notCapturedReason` is emitted alongside the captured value.

| Cause                                | Probe / scenario                                                | Expected in snapshot                                     | Regression check (must NOT appear) |
|--------------------------------------|------------------------------------------------------------------|----------------------------------------------------------|------------------------------------|
| `depth` (pointer-chasing limit)      | `MaxReferenceDepth: 2`, probe a struct with a 5-level pointer chain | `"notCapturedReason": "depth"` on the unreachable pointee | n/a — depth was correct before     |
| `tooManyPointersInFlight`            | Probe a struct with >128 pointer fields, all live                | `"notCapturedReason": "tooManyPointersInFlight"` on the first dropped field | `"depth"` on those fields          |
| `tooManyUniquePointers`              | Probe a graph with >1024 distinct addresses                      | `"notCapturedReason": "tooManyUniquePointers"`           | `"depth"`                          |
| `tooManySlicesCaptured`              | Probe a struct holding >128 distinct slices                      | `"notCapturedReason": "tooManySlicesCaptured"`           | `"depth"`                          |
| `captureNestingTooDeep` (per-field)  | Probe a struct nested past `ENQUEUE_STACK_DEPTH` (32 levels)     | `"notCapturedReason": "captureNestingTooDeep"` on the deepest reachable field | `"depth"`                          |
| `valueTooLarge`                      | Probe with `MaxLength: 16384` on a 10 KiB string                 | `"truncated": true` on the value (`notCapturedReason` is never emitted next to a captured value; the `size`/`truncated` pair already communicates the clamp) | no behavior change for partial captures |
| `stringSize`                         | Probe with `MaxLength: 32` on a 4 KiB string                     | `"truncated": true` on the value (same rule — `notCapturedReason` is reserved for values with no captured bytes) | no behavior change for partial captures |
| `collectionSize` (exactly-at-limit)  | Probe with `MaxCollectionSize: 50` on a slice of exactly 50 elements | `"notCapturedReason": "collectionSize"` on the slice block | no reason at all (today emits silence) |
| Event too large (fragment cap)       | Probe with arguments large enough to exceed 16 fragments × 32 KiB | `evaluationErrors[].expr == "@entry"` with message `event too large` on the affected side | `"depth"` on missing fields, no event-level reason |
| Agent overloaded (ringbuf rejection) | Stall the userspace ringbuf reader while a multi-fragment event is in flight | `evaluationErrors[].expr == "@entry"` (or `"@return"`) with message `agent overloaded` | `"depth"`                          |
| Return event lost                    | Force return-side first-flush failure (test hook in `pkg/dyninst/integration_first_flush_fail_test.go`) | `evaluationErrors[].expr == "@return"` with message `return event lost` | entry-only snapshot with no explanation |
| In-progress-calls map full           | Generate >8192 concurrent in-progress probed calls               | `evaluationErrors[].expr == "@return"` with message `in-progress-calls map full` | entry-only snapshot with no explanation |
| Per-goroutine call count exceeded    | One goroutine with >8 concurrent in-progress probed calls        | `evaluationErrors[].expr == "@return"` with message `per-goroutine call count exceeded` | entry-only snapshot with no explanation |
| Loss detail unknown (drop-notify overflow) | Saturate `drop_notify_ringbuf` while events are in flight, then wait past the grace window | `evaluationErrors[].expr == "@entry"` (or `"@return"`) with message `loss detail unknown` on whichever side is incomplete | event silently held until process shutdown |

## Aggregate counters (no JSON change)

After exercising the scenarios that trip condition evaluation
(`@when nil_ptr.field`, `@when slice[100]`, etc.), poll the agent's
internal stats endpoint and confirm:

- `condition_eval_error_nil_deref` increments on nil-deref conditions
- `condition_eval_error_other` increments on OOB / kernel-read-fail
  conditions

These counters are populated by `module.sink.HandleEvent` from the
event header's `Condition_eval_error` byte.

## Acceptance

The change is verified when every row above is reproducible and the
"regression check" string does not appear in the corresponding
snapshot. The single most important reproduction is the original
motivating case: a probe that captures a structure exceeding 16
fragments must no longer report missing fields as `"depth"` — they
should carry the per-side `eventTooLarge` reason or the per-field
reason inherited from a Carrier A placeholder peer.
