//go:build linux

// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_tracing.h>
#include <linux/sched.h>
#include <linux/in.h>
#include <profiler.h>

char LICENSE[] SEC("license") = "Dual BSD/GPL";

// Configuration injected by the Go user-space daemon at startup
int target_pid = 0;
unsigned long long target_dev = 0;
unsigned long long target_ino = 0;

// Ring buffer map connecting kernel space to Go user space
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024); // 256 KB
} events SEC(".maps");

// Helper: Validate container namespace context
static __always_inline int validate_namespace(void)
{
    if (target_dev == 0 || target_ino == 0)
        return 1; // If no container spec is provided, allow host-level profiling

    struct bpf_pidns_info ns;
    bpf_get_ns_current_pid_tgid(target_dev, target_ino, &ns, sizeof(ns));
    
    return (ns.pid == target_pid);
}

// Hook: Intercepting TCP Send Operations for Throughput and Latency
SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(kflow_tcp_sendmsg, struct sock *sk, struct msghdr *msg, size_t size)
{
    // 1. Filter out processes outside our target container namespace
    if (!validate_namespace())
        return 0;

    // 2. Reserve space in the ring buffer
    struct event_t *e;
    e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e)
        return 0;

    // 3. Extract socket family and network details safely using BPF CO-RE
    __u16 family = 0;
    BPF_CORE_READ_INTO(&family, sk, __sk_common.skc_family);

    if (family == AF_INET) {
        BPF_CORE_READ_INTO(&e->saddr, sk, __sk_common.skc_rcv_saddr);
        BPF_CORE_READ_INTO(&e->daddr, sk, __sk_common.skc_daddr);
        BPF_CORE_READ_INTO(&e->sport, sk, __sk_common.skc_num);
        
        __u16 dport = 0;
        BPF_CORE_READ_INTO(&dport, sk, __sk_common.skc_dport);
        e->dport = __builtin_bswap16(dport);
    }

    // 4. Populate event metrics
    struct bpf_pidns_info ns;
    bpf_get_ns_current_pid_tgid(target_dev, target_ino, &ns, sizeof(ns));
    e->pid = ns.pid;
    e->bytes = (__u64)size;
    e->duration_ns = bpf_ktime_get_ns(); // Timestamp / latency reference point

    // 5. Submit event to the ring buffer
    bpf_ringbuf_submit(e, 0);
    return 0;
}