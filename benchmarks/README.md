# pprof overhead benchmarks

These benchmarks quantify the runtime cost of *actively profiling* hotpod — not
the cost of the `_ "net/http/pprof"` blank import (which only registers HTTP
handlers) or of `startPprof()` serving on `:6060` (idle until scraped). The
overhead people actually care about comes from the profilers doing their
sampling work while the app runs:

- **CPU profiler** — delivers `SIGPROF` at 100 Hz and unwinds the stack of
  whatever is running. Measured against `burnCPU` (`internal/handlers/cpu.go`),
  a pure CPU workload with no I/O, so the signal-handling cost is isolated.
- **Block + mutex profilers** — instrument goroutine parking and lock
  contention. Measured against the queue's `Enqueue`/`Dequeue` path
  (`internal/queue/queue.go`), a single `sync.Mutex` hammered by
  `GOMAXPROCS` goroutines — the same hot spot the worker pool hits per item.
- **Heap profiler** — samples allocations (`runtime.MemProfileRate`). Measured
  against the allocate+fill core of the `/memory` handler
  (`internal/handlers/memory.go`), swept across allocation sizes. Note this
  profiler is *on by default* at 512 KiB in every Go program — see below.

Both workloads report a throughput metric (`iters/s`, `ops/s`) rather than
relying on `ns/op`: `burnCPU` is duration-bounded, so profiling shows up as
*fewer iterations completed in the same wall time*, not a longer op.

## Regenerating

```bash
make bench                 # writes benchmarks/pprof-overhead.txt
make bench BENCH_COUNT=10  # more samples for tighter estimates
```

Knobs: `BENCH_TIME` (default `1s`), `BENCH_COUNT` (default `6`), `BENCH_OUT`
(default `benchmarks/pprof-overhead.txt`).

If you have [`benchstat`](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat),
it pivots the profiling dimension into a side-by-side delta:

```bash
benchstat -col /pprof      benchmarks/pprof-overhead.txt   # CPU cases
benchstat -col /profile    benchmarks/pprof-overhead.txt   # queue cases
benchstat -col /memprofile benchmarks/pprof-overhead.txt   # memory cases
```

## Baseline measurement (one-time)

Captured 2026-07-07 on an Apple M3 Pro (11 logical CPUs), Go 1.26, `BENCH_COUNT=6`.
Raw output lives in [`pprof-overhead.txt`](./pprof-overhead.txt). Values are the
median across the 6 runs; overhead is the throughput drop from profiler-off to
profiler-on.

### CPU profiler (`burnCPU`)

| intensity | cores | off (iters/s) | on (iters/s) | overhead |
| --------- | ----- | ------------: | -----------: | -------: |
| low       | 1     | 6,998,266     | 6,678,240    | 4.6%     |
| low       | 4     | 5,535,658     | 5,427,737    | 2.0%     |
| medium    | 1     | 40,110        | 39,968       | 0.4%     |
| medium    | 4     | 153,236       | 149,642      | 2.3%     |
| high      | 1     | 2,608,084     | 2,528,554    | 3.0%     |
| high      | 4     | 9,545,856     | 9,356,686    | 2.0%     |

### Block + mutex profilers (queue contention)

| workload         | off (ops/s) | on (ops/s) | overhead |
| ---------------- | ----------: | ---------: | -------: |
| Enqueue+Dequeue  | 3,489,926   | 2,400,597  | 31.2%    |

### Heap profiler (allocate + fill)

Median ns/op; overhead is relative to `memprofile=off` (rate=0). `default` is
Go's built-in 512 KiB rate; `all` is rate=1 (sample every allocation).

| alloc size | off      | default (Δ)      | all (Δ)             |
| ---------- | -------: | ---------------: | ------------------: |
| 1 KiB      | 354.6 ns | 352.3 ns (~0%)   | 939.2 ns (**+165%**) |
| 64 KiB     | 19758 ns | 19795 ns (~0%)   | 20102 ns (+1.7%)    |
| 1 MiB      | 309561 ns| 310490 ns (~0%)  | 321679 ns (+3.9%)   |

## Reading the numbers

- **CPU profiling is cheap here: ~0.4–4.6%.** The `low` intensity path takes the
  biggest hit because it calls `runtime.Gosched()` on every iteration — frequent
  scheduler transitions give SIGPROF more to unwind and disrupt. The `medium`
  path (long tight math loops, few scheduling points) is almost unaffected.
- **Block + mutex profiling is expensive under contention: ~31%.** This is a
  worst case: the profilers are set to their most aggressive sampling
  (`SetBlockProfileRate(1)`, `SetMutexProfileFraction(1)` — record *every*
  event) against a workload that is nearly all lock traffic. Real services
  sample a fraction of events and do more work between locks, so production
  overhead is lower — but this shows why block/mutex profiling should be
  enabled deliberately and sampled, not left on at rate 1.
- **The default heap profiler is effectively free: ~0% at every size.** This
  matters because it is already running in your binary — disabling it
  (`MemProfileRate = 0`) buys you nothing measurable here. Sampling *every*
  allocation (rate=1) is a different story: it costs a fixed amount per
  allocation, so the hit is enormous for small, frequent allocations
  (**+165%** at 1 KiB) and fades as the allocation grows and that fixed cost is
  amortized over more fill work (a few percent at 64 KiB–1 MiB). The lesson:
  heap-profiler cost tracks allocation *count*, not bytes — leave it at the
  default rate unless you are chasing a specific allocation-heavy path.
- These are single-machine medians, not rigorous confidence intervals. For
  comparisons that matter, bump `BENCH_COUNT` and run `benchstat`.
