# Technical Specification: Go eBPF Container Traffic & Latency Profiler (kflow)

## 1. System Overview & Architecture
kflow is a low-overhead, zero-instrumentation systems daemon built in Go. It attaches to Linux kernel socket tracepoints via eBPF (Extended Berkeley Packet Filter) to measure network throughput, request latencies, and payload sizes across running containers in real time. It transfers raw events from kernel space to user space using a BPF Ring Buffer, where a concurrent Go pipeline processes, aggregates, and exports metrics to a Prometheus endpoint.
+-----------------------------------------------------------------------+
|                             KERNEL SPACE                              |
|                                                                       |
|   [ Socket Tracepoint ] ---> ( eBPF Program: kprobe/tp )              |
|                                       |                               |
|                                       v                               |
|                           [ BPF Ring Buffer Map ]                     |
+-----------------------------------------------------------------------+
                                        |  (Zero-Copy Event Stream)
========================================|================================
                                        v
+-----------------------------------------------------------------------+
|                            USER SPACE (Go)                            |
|                                                                       |
|                   +-------------------------------+                   |
|                   |   Ring Buffer Poller (1 GC)   |                   |
|                   +-------------------------------+                   |
|                                   | (Bounded Ring Channel)            |
|                                   v                                   |
|                   +-------------------------------+                   |
|                   |  Sharded Worker Pool (N CPUs) |                   |
|                   +-------------------------------+                   |
|                                   | (Atomic Mutex-Free Aggregation)   |
|                                   v                                   |
|                   +-------------------------------+                   |
|                   |   In-Memory Metric Collector  |                   |
|                   +-------------------------------+                   |
|                                   |                                   |
|                                   v                                   |
|                   +-------------------------------+                   |
|                   |   Prometheus HTTP /metrics    |                   |
|                   +-------------------------------+                   |
+-----------------------------------------------------------------------+

## 2. Kernel-Space Specification (eBPF / C)

### 2.1 Tracepoint Targets
The eBPF probe attaches to the Linux kernel network tracepoint:sys_enter_write / sys_exit_write (or kprobe/tcp_sendmsg and kretprobe/tcp_v4_connect) to capture outbound bytes, IP addresses, ports, and socket operation durations.

### 2.2 Event Data Structure (struct event_t)
The C eBPF program packs kernel telemetry into a fixed 32-byte C struct:

```c 
struct event_t {
    u32 pid;              // Process ID generating the network event
    u32 saddr;            // IPv4 Source Address
    u32 daddr;            // IPv4 Destination Address
    u16 sport;            // Source Port
    u16 dport;            // Destination Port
    u64 bytes;            // Payload size in bytes
    u64 duration_ns;      // Kernel processing duration in nanoseconds
};
```

### 2.3 Ring Buffer Configuration
- **Type**: BPF_MAP_TYPE_RINGBUF
- **Size**: 4 MB (4 * 1024 * 1024 bytes) to prevent dropped kernel events under multi-gigabit traffic spikes.

## 3. User-Space Specification (Go Concurrency Engine)

### 3.1 Event Ingestion (Zero-Allocation Loop)
- **Library**: [github.com/cilium/ebpf](https://github.com/cilium/ebpf)
- **Buffer Management**: A single dedicated goroutine continuously pulls raw bytes from ringbuf.Reader.Object 
- **Pooling**: Uses sync.Pool for event_t structs to avoid allocating memory on the Go heap during high-frequency ring buffer consumption.

### 3.2 Sharded Worker Pool Architecture
To eliminate lock contention when receiving up to 100,000 events/sec:
- **Worker Routing**: The ring buffer poller hashes the destination IP + Port (saddr ^ daddr) to route events deterministically to a specific worker channel (N = runtime.NumCPU()).
- **Lock-Free Processing**: Each worker maintains its own isolated, unshared Go map (map[string]*MetricSet), eliminating mutex locks during event updates.

                        [ Poller Goroutine ]
                               |
            +------------------+------------------+
            | (Hash: dest_ip)  | (Hash: dest_ip)  |
            v                  v                  v
     [ Channel 0 ]      [ Channel 1 ]      [ Channel N ]
            |                  |                  |
     [ Worker 0 ]       [ Worker 1 ]       [ Worker N ]
            |                  |                  |
     (Local Map 0)      (Local Map 1)      (Local Map N)

### 3.3 Metric Aggregation & Flush Cycle
- **Flush Ticker**: A background time.Ticker fires every 1 second. 
- **Worker Snapshot**: Workers swap their active local map pointer with a fresh empty map and flush the aggregated metrics to the global Prometheus collector. 
- **Metrics Exposed**:
    - kflow_packets_total{src_ip, dst_ip, dst_port} (Counter)
    - kflow_bytes_total{src_ip, dst_ip, dst_port} (Counter)
    - kflow_latency_nanoseconds_bucket{src_ip, dst_ip, dst_port, le} (Histogram)

## 4. Technical Stack & Prerequisites
- **OS**: Linux Kernel ≥ 5.8 (required for BPF_MAP_TYPE_RINGBUF support)
- **Language**: Go ≥ 1.22 + C (eBPF profile)
- **Compiler / Toolchain**: clang, llvm, libbpf-dev, bpf2go
- **Observability**: prometheus/client_golang for metrics export
- **Deployment Context**: Privileged Docker container or bare-metal Linux node with CAP_BPF and - CAP_PERFMON (or CAP_SYS_ADMIN) capabilities

## 5. File Structure & Implementation Blueprintkflow/
├── cmd/
│   └── main.go               # Program entrypoint, OS signal handling, HTTP metrics server
├── ebpf/
│   ├── profiler.c            # C eBPF kernel program
│   ├── profiler.h            # Data structures shared between C and Go
│   └── generate.go           # //go:generate directive for bpf2go compilation
├── internal/
│   ├── collector/
│   │   └── prometheus.go     # Prometheus registry and metric definitions
│   ├── pipeline/
│   │   ├── pool.go           # sync.Pool for zero-allocation struct re-use
│   │   ├── poller.go         # eBPF ring buffer reader
│   │   └── worker_pool.go    # Sharded worker channels and lock-free aggregation
│   └── sys/
│       └── kernel.go         # Capability checks and rlimit unlocker
├── Makefile                  # Build targets for clang, bpf2go, and Go binaries
└── go.mod

## 6. Verification & Stress Testing Plan
- **Kernel Loading Test**: Verify the C eBPF program compiles via `bpf2go` and loads into kernel space without triggering the eBPF verifier error (`BPF verifier rejected program`).
- **Synthetic Traffic Spike**: Generate high-frequency concurrent traffic using `hey` or `wrk` against a dummy HTTP container on port 8080:

  ```bash
  hey -z 30s -c 200 -q 1000 http://localhost:8080/
  ```
- **Performance Profiling**:
    - Measure kflow CPU and RAM consumption using pprof under 100k events/second.
    - Verify zero GC heap allocations in the hot path via go test -bench . -benchmem.
- **Prometheus Scraping**: Validate metrics stream live at http://localhost:9090/metrics.