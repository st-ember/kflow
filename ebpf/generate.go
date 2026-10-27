package ebpf

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -type event_t bpf profiler.c -- -I/usr/include
