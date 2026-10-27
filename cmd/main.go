package main

import (
	"log"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/st-ember/kflow/internal/collector"
	"github.com/st-ember/kflow/internal/pipeline"
)

func main() {
	// Metrics
	m := collector.NewMetrics()

	// Worker Pool
	wp := pipeline.NewWorkerPool(8, m)
	wp.Start()

	// Poller
	rd, err := ringbuf.NewReader(&ebpf.Map{})
	if err != nil {
		log.Fatalf("Cannot initialize ringbuf reader: %v", err)
	}

	p := pipeline.NewPoller(rd, wp.WorkerChans())
	p.Start()
}
