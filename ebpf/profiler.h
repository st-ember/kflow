#ifndef __PROFILER_H__
#define __PROFILER_H__

#include <linux/types.h>

// Standardized 32-byte event schema for Go consumption
struct event_t {
    __u32 pid;
    __u32 saddr;
    __u32 daddr;
    __u16 sport;
    __u16 dport;
    __u64 bytes;
    __u64 duration_ns;
};

#endif /* __PROFILER_H__ */