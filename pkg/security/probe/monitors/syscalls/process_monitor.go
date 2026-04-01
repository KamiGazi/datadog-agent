// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build linux

// Package syscalls holds syscalls related files
package syscalls

import (
	"fmt"
	"time"

	manager "github.com/DataDog/ebpf-manager"
	lib "github.com/cilium/ebpf"

	"github.com/DataDog/datadog-agent/pkg/security/ebpf"
	"github.com/DataDog/datadog-agent/pkg/security/probe/managerhelper"
	"github.com/DataDog/datadog-agent/pkg/security/resolvers/process"
	"github.com/DataDog/datadog-agent/pkg/security/secl/model"
	"github.com/DataDog/datadog-agent/pkg/security/seclog"
	"github.com/DataDog/datadog-agent/pkg/security/utils"
)

// syscallMonitorKey matches struct syscall_monitor_key_t { u32 idx; u32 pid; }
// idx is the bucket index (syscall_id / 64) and pid is the process PID.
type syscallMonitorKey struct {
	Idx uint32
	Pid uint32
}

// ProcessMonitor dispatches one custom event per cgroup that has observed syscalls
type ProcessMonitor struct {
	syscallMonitor  [2]*lib.Map
	bufferSelector  *lib.Map
	activeBuffer    uint32
	processResolver *process.EBPFResolver
	numCPU          int
	Period          time.Duration
	lastSent        time.Time
	newEventFnc     func() *model.Event
}

// SendEvents iterates the active syscall monitor map and dispatches one custom event
// per cgroup. Keys are (bucket_idx, pid) and values are per-CPU u64 bitmasks where
// bit N represents syscall (bucket_idx*64 + N). It is a no-op until Period has elapsed
// since the previous send.
func (d *ProcessMonitor) SendEvents(dispatchFn func(*model.Event)) error {
	now := time.Now()
	if d.Period > 0 && now.Sub(d.lastSent) < d.Period {
		return nil
	}
	d.lastSent = now

	// swap buffers: tell eBPF to write into the other buffer from now on,
	// then drain the buffer it just vacated.
	d.activeBuffer = 1 - d.activeBuffer
	if err := d.bufferSelector.Put(ebpf.BufferSelectorSyscallMonitorKey, d.activeBuffer); err != nil {
		return fmt.Errorf("failed to swap syscall monitor buffer: %w", err)
	}

	buffer := d.syscallMonitor[1-d.activeBuffer]

	// pidSyscalls accumulates decoded syscall IDs per PID.
	pidSyscalls := make(map[uint32][]model.Syscall)
	cpuValues := make([]uint64, d.numCPU)

	var key syscallMonitorKey
	iterator := buffer.Iterate()
	for iterator.Next(&key, &cpuValues) {
		// OR together all per-CPU bitmasks to get the complete set for this (idx, pid).
		var combined uint64
		for _, v := range cpuValues {
			combined |= v
		}
		if combined == 0 {
			continue
		}

		// Decode the 64-bit bitmask into individual syscall IDs.
		for bit := uint64(0); bit < 64; bit++ {
			if combined&(1<<bit) != 0 {
				syscallID := uint64(key.Idx)*64 + bit
				pidSyscalls[key.Pid] = append(pidSyscalls[key.Pid], model.Syscall(syscallID))
				seclog.Tracef("syscall monitor: pid %d observed syscall %d", key.Pid, syscallID)
			}
		}
	}
	if err := iterator.Err(); err != nil {
		return fmt.Errorf("syscall monitor map iteration failed: %w", err)
	}

	seclog.Debugf("syscall monitor: found %d pids with syscalls", len(pidSyscalls))

	for pid, syscalls := range pidSyscalls {
		if len(syscalls) == 0 {
			seclog.Tracef("no syscalls for pid %d", pid)
			continue
		}

		pce := d.processResolver.Resolve(pid, pid, 0, false, nil)
		if pce == nil || pce.ContainerContext.IsNull() {
			continue
		}

		ev := d.newEventFnc()
		ev.Type = uint32(model.SyscallsEventType)
		ev.ProcessCacheEntry = pce
		ev.ProcessContext = &pce.ProcessContext
		ev.Syscalls = model.SyscallsEvent{
			Syscalls: syscalls,
		}
		dispatchFn(ev)
	}

	return nil
}

// NewProcessMonitor returns a new ProcessMonitor
func NewEBPFProcessMonitor(mgr *manager.Manager, resolver *process.EBPFResolver, newEventFnc func() *model.Event, period time.Duration) (*ProcessMonitor, error) {
	numCPU, err := utils.NumCPU()
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch the host CPU count: %w", err)
	}

	monitor := &ProcessMonitor{
		processResolver: resolver,
		numCPU:          numCPU,
		Period:          period,
		newEventFnc:     newEventFnc,
	}

	fbSyscallMonitor, err := managerhelper.Map(mgr, "fb_syscall_monitor")
	if err != nil {
		return nil, err
	}
	monitor.syscallMonitor[0] = fbSyscallMonitor

	bbSyscallMonitor, err := managerhelper.Map(mgr, "bb_syscall_monitor")
	if err != nil {
		return nil, err
	}
	monitor.syscallMonitor[1] = bbSyscallMonitor

	bufferSelector, err := managerhelper.Map(mgr, "buffer_selector")
	if err != nil {
		return nil, err
	}
	monitor.bufferSelector = bufferSelector

	return monitor, nil
}
