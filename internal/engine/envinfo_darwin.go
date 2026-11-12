//go:build darwin

package engine

import (
	"time"

	"golang.org/x/sys/unix"
)

func readEnvSample() envSample {
	var s envSample

	var ru unix.Rusage
	if err := unix.Getrusage(unix.RUSAGE_SELF, &ru); err == nil {
		s.cpuUser = time.Duration(ru.Utime.Sec)*time.Second +
			time.Duration(ru.Utime.Usec)*time.Microsecond
		s.cpuSys = time.Duration(ru.Stime.Sec)*time.Second +
			time.Duration(ru.Stime.Usec)*time.Microsecond
		s.cpuOK = true
		if ru.Maxrss > 0 {
			// macOS reports max RSS in bytes.
			s.maxRSSBytes = uint64(ru.Maxrss)
		}
	}

	if n, err := unix.SysctlUint64("hw.memsize"); err == nil {
		s.memTotal = n
	}
	if s.memTotal > 0 {
		if avail, err := unix.SysctlUint64("vm.page_free_count"); err == nil {
			pageSize, err := unix.SysctlUint64("hw.pagesize")
			if err != nil || pageSize == 0 {
				pageSize = 4096
			}
			s.memAvail = avail * pageSize
		}
	}
	if kernel, err := unix.Sysctl("kern.osrelease"); err == nil {
		s.kernel = kernel
	}

	return s
}
