package collector

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/st-ember/kflow/internal/pipeline"
)

type PromMetrics struct {
	PacketsTotal       *prometheus.CounterVec
	BytesTotal         *prometheus.CounterVec
	LatencyNanoseconds *prometheus.HistogramVec
}

func NewMetrics() pipeline.Metrics {
	return &PromMetrics{
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
				Name:    "kflow_latency_nanoseconds",
				Help:    "Kernel processing duration in nanoseconds for network operations.",
				Buckets: prometheus.ExponentialBuckets(1000, 10, 6),
			},
			[]string{"src_ip", "dst_ip", "dst_port"},
		),
	}
}

func (m *PromMetrics) BatchIncPacket(srcIP, dstIP, dstPort string, incTimes float64) {
	m.PacketsTotal.WithLabelValues(srcIP, dstIP, dstPort).Add(incTimes)
}

func (m *PromMetrics) BatchIncByte(srcIP, dstIP, dstPort string, incTimes float64) {
	m.BytesTotal.WithLabelValues(srcIP, dstIP, dstPort).Add(incTimes)
}

func (m *PromMetrics) ObserveLatencyNS(srcIP, dstIP, dstPort string, latencyNS float64) {
	m.LatencyNanoseconds.WithLabelValues(srcIP, dstIP, dstPort).Observe(latencyNS)
}
