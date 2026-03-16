package worker

import (
	"context"
	"log"

	"Pulsemon/internal/scheduler"
)

// WorkerPool runs N goroutines that consume ProbeJobs and produce ProbeResults.
type WorkerPool struct {
	jobs        <-chan scheduler.ProbeJob
	results     chan<- ProbeResult
	workerCount int
	prober      Prober
}

// NewWorkerPool creates a WorkerPool.
func NewWorkerPool(
	jobs <-chan scheduler.ProbeJob,
	results chan<- ProbeResult,
	workerCount int,
	prober Prober,
) *WorkerPool {
	return &WorkerPool{
		jobs:        jobs,
		results:     results,
		workerCount: workerCount,
		prober:      prober,
	}
}

// Start spawns workerCount goroutines that process ProbeJobs until the
// context is cancelled or the jobs channel is closed.
func (wp *WorkerPool) Start(ctx context.Context) {
	log.Printf("worker pool: starting %d workers", wp.workerCount)

	for i := 0; i < wp.workerCount; i++ {
		go func(id int) {
			for {
				select {
				case job, ok := <-wp.jobs:
					if !ok {
						return // channel closed, shutdown
					}
					result := wp.prober.Probe(ctx, job)
					wp.results <- result
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}
}
