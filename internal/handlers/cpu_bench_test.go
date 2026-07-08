package handlers

import (
	"context"
	"io"
	"runtime/pprof"
	"strconv"
	"testing"
	"time"
)

// benchCPUQuantum is the wall-clock duration each burnCPU invocation runs for
// during benchmarking. burnCPU is duration-bounded (it stops when its context
// deadline fires), so a "faster" run does not finish sooner — it completes more
// iterations in the same wall time. Throughput is therefore reported as
// iters/s rather than read off the default ns/op.
const benchCPUQuantum = 50 * time.Millisecond

// BenchmarkBurnCPUProfileOverhead measures the throughput cost of running the
// Go CPU profiler (SIGPROF sampling at 100 Hz) while burnCPU executes.
//
// Each workload runs twice — once with the CPU profiler disabled (pprof=off)
// and once with it enabled (pprof=on) — across the three intensity levels and
// two goroutine fan-outs. The drop in iters/s from pprof=off to pprof=on is the
// profiling overhead. The cores dimension exists because SIGPROF is delivered
// per running OS thread, so the cost is expected to grow with parallelism.
//
// Compare the pairs directly, e.g. with benchstat pivoting on the pprof label:
//
//	benchstat -col /pprof benchmarks/pprof-overhead.txt
func BenchmarkBurnCPUProfileOverhead(b *testing.B) {
	intensities := []string{intensityLow, intensityMedium, intensityHigh}
	coreCounts := []int{1, 4}

	for _, intensity := range intensities {
		for _, cores := range coreCounts {
			prefix := "intensity=" + intensity + "/cores=" + strconv.Itoa(cores)
			b.Run(prefix+"/pprof=off", func(b *testing.B) {
				runBurnCPUBench(b, intensity, cores, false)
			})
			b.Run(prefix+"/pprof=on", func(b *testing.B) {
				runBurnCPUBench(b, intensity, cores, true)
			})
		}
	}
}

func runBurnCPUBench(b *testing.B, intensity string, cores int, profile bool) {
	if profile {
		// Discard the samples: we are measuring the cost of collecting them,
		// not analyzing them.
		if err := pprof.StartCPUProfile(io.Discard); err != nil {
			b.Fatalf("start cpu profile: %v", err)
		}
		defer pprof.StopCPUProfile()
	}

	ctx := context.Background()
	var totalIters int64

	b.ResetTimer()
	for range b.N {
		iters, _ := burnCPU(ctx, benchCPUQuantum, cores, intensity)
		totalIters += iters
	}
	b.StopTimer()

	if secs := b.Elapsed().Seconds(); secs > 0 {
		b.ReportMetric(float64(totalIters)/secs, "iters/s")
		b.ReportMetric(float64(totalIters)/float64(b.N), "iters/op")
	}
}
