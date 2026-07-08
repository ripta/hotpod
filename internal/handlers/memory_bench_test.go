package handlers

import (
	"runtime"
	"testing"
)

// defaultMemProfileRate is Go's built-in default for runtime.MemProfileRate:
// one sampled allocation per ~512 KiB allocated.
const defaultMemProfileRate = 512 * 1024

// allocFillSink defeats dead-store elimination so the allocation and fill in
// the benchmark loop cannot be optimized away.
var allocFillSink []byte

// BenchmarkAllocFillProfileOverhead measures the throughput cost of the Go heap
// profiler while allocating and filling memory.
//
// The heap profiler is unusual: it is ON BY DEFAULT in every Go program at
// runtime.MemProfileRate = 512 KiB. "Enabling pprof" does not start it — it is
// already running. So this benchmark answers two different questions:
//
//   - memprofile=off (rate=0) vs memprofile=default (512 KiB): the cost of the
//     always-present heap sampling you are already paying in production.
//   - memprofile=default vs memprofile=all (rate=1, sample every allocation):
//     the extra cost of maximal sampling.
//
// Overhead is paid per sampled allocation *event*, and the sampling probability
// for a fixed rate rises with allocation size. Many small allocations at rate=1
// is the worst case; a handful of large ones is sampled at essentially prob 1
// even at the default rate, so default and all converge there. The size sweep
// makes that trade-off visible.
//
// The loop mirrors holdMemory's allocate+fill core (make + fillMemory,
// memory.go). The hold/timer tail of holdMemory is omitted: it neither
// allocates meaningfully nor is what the heap profiler instruments, and its
// wakeup latency would swamp the small-allocation cases. Throughput is read off
// the default MB/s (via SetBytes); allocs/op comes from ReportAllocs.
//
//	benchstat -col /memprofile benchmarks/pprof-overhead.txt
func BenchmarkAllocFillProfileOverhead(b *testing.B) {
	sizes := []int64{1 << 10, 64 << 10, 1 << 20} // 1 KiB, 64 KiB, 1 MiB
	rates := []struct {
		label string
		rate  int
	}{
		{"memprofile=off", 0},
		{"memprofile=default", defaultMemProfileRate},
		{"memprofile=all", 1},
	}

	for _, size := range sizes {
		for _, r := range rates {
			b.Run("size="+formatSize(size)+"/"+r.label, func(b *testing.B) {
				runAllocFillBench(b, size, r.rate)
			})
		}
	}
}

func runAllocFillBench(b *testing.B, size int64, memProfileRate int) {
	// MemProfileRate is process-global; restore it so later benchmarks and the
	// test binary's own final heap dump are unaffected.
	prev := runtime.MemProfileRate
	runtime.MemProfileRate = memProfileRate
	defer func() { runtime.MemProfileRate = prev }()

	b.SetBytes(size)
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		buf := make([]byte, size)
		// Sequential (not random) fill: a plain write loop keeps the
		// allocation, not RNG, as the dominant per-op cost so the profiler
		// overhead is not diluted.
		fillMemory(buf, patternSequential)
		allocFillSink = buf
	}
}
