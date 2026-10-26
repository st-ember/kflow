package collector

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	PacketsTotal *prometheus.CounterVec
	BytesTotal *prometheus.CounterVec
	LatencyNanoseconds *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		PacketsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "kflow_packets_total",
				Help: "Total packets transmited across container sockets.",
			},
			[]string{"src_ip", "dst_ip", "dst_port"},
		),
		BytesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "kflow_bytes_total",
				Help: "Total payload bytes transmitted across container sockerts.",
			},
			[]string{"src_ip", "dst_ip", "dst_port"},
		),
		LatencyNanoseconds: promauto.NewHistogramVec(
prometheus.HistogramOpts{
	Name: "kflow_latency_nanoseconds",
	Help: "Kernel processing duration in nanoseconds for network operations.",
	Buckets: prometheus.ExponentialBuckets(1000, 10, 6),
},
	[]string{"src_ip", "dst_ip", "dst_port"},		
		),
	}
}
