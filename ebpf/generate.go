package ebpf

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -type event_t Bpf profiler.c -- -I/usr/include -D__TARGET_ARCH_x86 -Wno-missing-declarations
