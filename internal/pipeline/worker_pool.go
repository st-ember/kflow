package pipeline

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/st-ember/kflow/internal/collector"
)

type MetricSet struct {
	Packets uint64
	Bytes uint64
	Durations []uint64
}

type Worker struct {
	id int
	inputChan chan *Event
	localMap map[string]*MetricSet
	stopChan chan struct{}
}

type WorkerPool struct {
	workers []*Worker
	workerChans []chan *Event
}

func NewWorkerPool(numWorkers int) *WorkerPool {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}

	workers:= make([]*Worker, numWorkers)
	workerChans := make([]chan *Event, numWorkers)

	for i := range numWorkers {
		workerChans[i] = make(chan *Event, 4096)
		workers[i] = &Worker{
			id: i,
			inputChan: workerChans[i],
			localMap: make(map[string]*MetricSet),
			stopChan: make(chan struct{}),
		}
	}

	return &WorkerPool{
		workers: workers,
		workerChans: workerChans,
	}
}

func (wp *WorkerPool) WorkerChans() []chan *Event {
	return wp.workerChans
}

func (wp *WorkerPool) Start() {
	for _, w := range wp.workers {
		go w.Run()
	}
	log.Printf("[WorkerPool] Launched %d sharded worker goroutines.", len(wp.workers))
}

func (wp *WorkerPool) Stop() {
	for _, w := range wp.workers {
		close(w.stopChan)
	}
}

func (w *Worker) run() {
	flushTicker := time.NewTicker(1 * time.Second)
	defer flushTicker.Stop()

	for {
		select {
		case event, ok := <-w.inputChan:
			if !ok {
				return
			}

			// Construct composite map key
			saddrStr := formatIP(event.SAddr)
			daddrStr := formatIP(event.DAddr)
			dportStr := fmt.Sprintf("%d", event.DPort)
			mapKeys := fmt.Sprintf("%s|%s|%s", saddrStr, daddrStr, dportStr)

			// Find or create metric
			metric, exists := w.localMap[mapKeys]
			if !exists {
				metric = &MetricSet{}
				w.localMap[mapKeys] = metric
			}

			metric.Packets++
			metric.Bytes += event.Bytes
			metric.Durations = append(metric.Durations, event.DurationNs)

			// Recycle event back to sync.WorkerPool
			ReleaseEvent(event)
		case <-flushTicker.C:
			w.flushMetrics()

		case <-w.stopChan:
			w.flushMetrics()
			return
		}
	}
}

func (w *Worker) flushMetrics() {
	if len(w.localMap) == 0 {
		return
	}

	for key, metric := range w.localMap {
		var saddr, daddr, dport string
		_, _ = fmt.Scanf(key, "%s|%s|%s", &saddr, &daddr, &dport)

		// collector.
	}

	// Reset local localMap
	w.localMap = make(map[string]*MetricSet)
}

func (w *Worker) formatIP(ip uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d",byte(ip), byte(ip>>8), byte(ip>>16), byte(ip>>24))
}
