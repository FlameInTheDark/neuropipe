//go:build windows

package metrics

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var getProcessMemoryInfo = windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")

type processMemoryCounters struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

type processSampleState struct {
	cpuAt time.Time
	cpuNS int64
}

type windowsProcessSampler struct {
	mu    sync.Mutex
	state map[int]processSampleState
}

// NewProcessSampler creates the Windows-only collector for processes owned by
// Neuropipe. It never enumerates or samples unrelated processes.
func NewProcessSampler() ProcessSampler {
	return &windowsProcessSampler{state: make(map[int]processSampleState)}
}

func (s *windowsProcessSampler) Sample(pid int) (float64, int64, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_VM_READ, false, uint32(pid))
	if err != nil {
		return 0, 0, fmt.Errorf("open owned process %d: %w", pid, err)
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		return 0, 0, fmt.Errorf("read owned process time: %w", err)
	}
	memory := processMemoryCounters{CB: uint32(unsafe.Sizeof(processMemoryCounters{}))}
	result, _, callErr := syscall.SyscallN(getProcessMemoryInfo.Addr(), uintptr(handle), uintptr(unsafe.Pointer(&memory)), uintptr(memory.CB))
	if result == 0 {
		return 0, 0, fmt.Errorf("read owned process memory: %w", callErr)
	}
	now := time.Now()
	cpuNS := kernel.Nanoseconds() + user.Nanoseconds()
	cpuPercent := 0.0
	s.mu.Lock()
	if previous, exists := s.state[pid]; exists {
		wall := now.Sub(previous.cpuAt)
		if wall > 0 && cpuNS >= previous.cpuNS {
			cpuPercent = float64(cpuNS-previous.cpuNS) / float64(wall) * 100 / float64(runtime.NumCPU())
		}
	}
	s.state[pid] = processSampleState{cpuAt: now, cpuNS: cpuNS}
	s.mu.Unlock()
	return cpuPercent, int64(memory.WorkingSetSize), nil
}
