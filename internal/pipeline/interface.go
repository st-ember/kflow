package pipeline

type Metrics interface {
	BatchIncPacket(srcIP, dstIP, dstPort string, incTimes float64)
	BatchIncByte(srcIP, dstIP, dstPort string, incTimes float64)
	ObserveLatencyNS(srcIP, dstIP, dstPort string, latencyNS float64)
}
