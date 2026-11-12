//go:build windows

package engine

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modkernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx = modkernel32.NewProc("GlobalMemoryStatusEx")

	modpsapi                 = windows.NewLazySystemDLL("psapi.dll")
	procGetProcessMemoryInfo = modpsapi.NewProc("GetProcessMemoryInfo")
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

type processMemoryCounters struct {
	CB                      uint32
	PageFaultCount          uint32
	PeakWorkingSetSize      uintptr
	WorkingSetSize          uintptr
	QuotaPeakPagedPoolUsage uintptr
	QuotaPagedPoolUsage     uintptr
	QuotaPeakNonPagedPool   uintptr
	QuotaNonPagedPool       uintptr
	PagefileUsage           uintptr
	PeakPagefileUsage       uintptr
}

func readEnvSample() envSample {
	var s envSample

	if handle, err := windows.GetCurrentProcess(); err == nil {
		var creation, exit, kernel, user windows.Filetime
		if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err == nil {
			s.cpuUser = time.Duration(user.Nanoseconds())
			s.cpuSys = time.Duration(kernel.Nanoseconds())
			s.cpuOK = true
		}

		var mem processMemoryCounters
		mem.CB = uint32(unsafe.Sizeof(mem))
		r1, _, _ := procGetProcessMemoryInfo.Call(
			uintptr(handle),
			uintptr(unsafe.Pointer(&mem)),
			uintptr(mem.CB),
		)
		if r1 != 0 {
			s.maxRSSBytes = uint64(mem.PeakWorkingSetSize)
		}
	}

	var ms memoryStatusEx
	ms.Length = uint32(unsafe.Sizeof(ms))
	if r1, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms))); r1 != 0 {
		s.memTotal = ms.TotalPhys
		s.memAvail = ms.AvailPhys
	}

	return s
}
