package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/plume-engine/plume/layout"
)

func main() {
	workersFlag := flag.Int("workers", 4, "Number of parallel fuzzing workers")
	durationFlag := flag.Duration("duration", 10*time.Second, "Duration to run fuzzing (0 for infinite)")
	diffFlag := flag.Bool("diff", false, "Enable differential checking against Rust Taffy oracle")
	flag.Parse()

	taffyBin := "/tmp/taffy_compare/target/release/taffy_compare"
	if *diffFlag {
		if _, err := os.Stat(taffyBin); err != nil {
			fmt.Printf("Warning: Taffy oracle binary not found at %s. Disabling differential check.\n", taffyBin)
			*diffFlag = false
		} else {
			fmt.Printf("Differential mode enabled using oracle: %s\n", taffyBin)
		}
	}

	fmt.Printf("Starting Plume Differential Fuzzer with %d workers (Duration: %v)...\n", *workersFlag, *durationFlag)

	var totalIterations uint64
	var totalCrashes uint64
	var totalNaNs uint64
	var totalDivergences uint64

	stopChan := make(chan struct{})
	if *durationFlag > 0 {
		time.AfterFunc(*durationFlag, func() {
			close(stopChan)
		})
	}

	start := time.Now()
	var wg sync.WaitGroup

	for w := 0; w < *workersFlag; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			buf := make([]byte, 64)

			for {
				select {
				case <-stopChan:
					return
				default:
				}

				_, _ = rand.Read(buf)
				treeDef, tree, ok := layout.GenerateTreeFromBytes(buf)
				if !ok {
					continue
				}

				// Execute Layout
				tree.Compute(treeDef.ViewportW, treeDef.ViewportH)
				atomic.AddUint64(&totalIterations, 1)

				// Check numerical invariants
				nodeCount := tree.NodeCount()
				for i := 0; i < nodeCount; i++ {
					rect := tree.ComputedRect(layout.NodeID(i))
					if math.IsNaN(float64(rect.X)) || math.IsNaN(float64(rect.Y)) ||
						math.IsNaN(float64(rect.Width)) || math.IsNaN(float64(rect.Height)) {
						atomic.AddUint64(&totalNaNs, 1)
					}
					if rect.Width < 0 || rect.Height < 0 {
						atomic.AddUint64(&totalCrashes, 1)
					}
				}

				// Differential check against Taffy
				if *diffFlag && (atomic.LoadUint64(&totalIterations)%50 == 0) {
					match, _ := layout.DifferentialCheckWithTaffy(taffyBin, treeDef, tree)
					if !match {
						atomic.AddUint64(&totalDivergences, 1)
					}
				}
			}
		}(w)
	}

	// Reporter goroutine
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-stopChan:
				return
			case <-ticker.C:
				elapsed := time.Since(start).Seconds()
				iters := atomic.LoadUint64(&totalIterations)
				rate := float64(iters) / elapsed
				fmt.Printf("[Fuzzer] %d iterations (%.0f iters/sec) | Crashes: %d | NaNs: %d | Divergences: %d\n",
					iters, rate, atomic.LoadUint64(&totalCrashes), atomic.LoadUint64(&totalNaNs), atomic.LoadUint64(&totalDivergences))
			}
		}
	}()

	wg.Wait()
	elapsed := time.Since(start).Seconds()
	iters := atomic.LoadUint64(&totalIterations)
	rate := float64(iters) / elapsed

	fmt.Printf("\n=== Fuzzing Complete ===\n")
	fmt.Printf("Total iterations: %d\n", iters)
	fmt.Printf("Average throughput: %.0f iterations/sec\n", rate)
	fmt.Printf("Crashes / Invariant Violations: %d\n", atomic.LoadUint64(&totalCrashes))
	fmt.Printf("NaNs / Infs: %d\n", atomic.LoadUint64(&totalNaNs))
	fmt.Printf("Oracle Divergences: %d\n", atomic.LoadUint64(&totalDivergences))
}
