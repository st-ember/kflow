package pipeline

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"

	"github.com/cilium/ebpf/ringbuf"
)

type Poller struct {
	reader      *ringbuf.Reader
	workerChans []chan *Event
	stopChan    chan struct{}
}

func NewPoller(rd *ringbuf.Reader, workerChans []chan *Event) *Poller {
	return &Poller{
		reader:      rd,
		workerChans: workerChans,
		stopChan:    make(chan struct{}),
	}
}

func (p *Poller) Start() {
	go func() {
		log.Println("[Poler] Ring buffer polling loop started.")
		for {
			select {
			case <-p.stopChan:
				log.Println("[Poller] Stopping ring buffer poller...")
				return
			default:
				// Pull raw bytes from PBF ring buffer (blocking read)
				record, err := p.reader.Read()
				if err != nil {
					if err == ringbuf.ErrClosed || err == io.EOF {
						return
					}
					log.Printf("[Poller] Read error: %v", err)
					continue
				}

				// Acquire an *Event from sync.Pool
				event := AcquireEvent()

				// Unmarshal raw binary buffer into the Event struct
				buf := bytes.NewReader(record.RawSample)
				if err := binary.Read(buf, binary.LittleEndian, event); err != nil {
					log.Printf("[Poller] Binary unmarshal failed: %v", err)
					ReleaseEvent(event)
					continue
				}

				// Deterministic Sharding Hash: XOR source and destination IPs
				hash := event.SAddr ^ event.DAddr
				workerIdx := hash % uint32(len(p.workerChans))

				// Dispatch event to the selected worker's unshared channel
				p.workerChans[workerIdx] <- event
			}

		}
	}()
}

func (p *Poller) Stop() {
	close(p.stopChan)
	if err := p.reader.Close(); err != nil {
		log.Printf("[Poller] Error closing ring buffer reader: %v", err)
	}
}
