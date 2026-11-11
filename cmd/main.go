package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	kebpf "github.com/st-ember/kflow/ebpf"
	"github.com/st-ember/kflow/internal/collector"
	"github.com/st-ember/kflow/internal/pipeline"
)

func main() {
	// Setup Signal-aware Context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Metrics
	m := collector.NewMetrics()

	// Worker Pool
	wp := pipeline.NewWorkerPool(8, m)
	wp.Start(ctx)

	// Remove Memlock limits
	if err := rlimit.RemoveMemlock(); err != nil {
		log.Fatalf("removing memlock: %v", err)
	}

	// BPF objects
	var objs kebpf.BpfObjects
	if err := kebpf.LoadBpfObjects(&objs, nil); err != nil {
		log.Fatalf("loading objects: %v", err)
	}
	defer objs.Close()

	// Kprobe
	kp, err := link.Kprobe("tcp_sendmsg", objs.KflowTcpSendmsg, nil)
	if err != nil {
		log.Fatalf("attaching kprobe: %v", err)
	}
	defer kp.Close()

	rd, err := ringbuf.NewReader(objs.Events)
	if err != nil {
		log.Fatalf("Cannot initialize ringbuf reader: %v", err)
	}

	// Ringbuf poller
	p := pipeline.NewPoller(rd, wp.WorkerChans())
	p.Start()

	// Expose prometheus
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:    ":9090",
		Handler: mux,
	}

	// Start HTTP server
	go func() {
		log.Printf("[Metrics] Starting Prometheus server on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Metrics] HTTP server failed: %v", err)
		}
	}()

	log.Println("Application started. Press Ctrl+C to exit.")

	// Block main() from exiting until the context is canceled
	<-ctx.Done()

	// Stop worker pool
	wp.Stop()

	log.Println("Shutdown signal received, shutting down gracefully...")
}
