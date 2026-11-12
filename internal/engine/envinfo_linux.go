//go:build linux || android

package engine

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func readEnvSample() envSample {
	var s envSample

	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err == nil {
		s.cpuUser = time.Duration(int64(ru.Utime.Sec))*time.Second +
			time.Duration(int64(ru.Utime.Usec))*time.Microsecond
		s.cpuSys = time.Duration(int64(ru.Stime.Sec))*time.Second +
			time.Duration(int64(ru.Stime.Usec))*time.Microsecond
		s.cpuOK = true
		if ru.Maxrss > 0 {
			// Linux and Android report max RSS in kilobytes.
			s.maxRSSBytes = uint64(ru.Maxrss) * 1024
		}
	}

	s.memTotal, s.memAvail = readMeminfo()
	s.load1, s.load5, s.load15 = readLoadavg()
	s.kernel = readKernel()

	return s
}

func readMeminfo() (total, available uint64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	var free, buffers, cached uint64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			total = v * 1024
		case "MemAvailable:":
			available = v * 1024
		case "MemFree:":
			free = v * 1024
		case "Buffers:":
			buffers = v * 1024
		case "Cached:":
			cached = v * 1024
		}
	}
	if available == 0 {
		available = free + buffers + cached
	}
	return total, available
}

func readLoadavg() (l1, l5, l15 float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0
	}
	l1, _ = strconv.ParseFloat(fields[0], 64)
	l5, _ = strconv.ParseFloat(fields[1], 64)
	l15, _ = strconv.ParseFloat(fields[2], 64)
	return l1, l5, l15
}

func readKernel() string {
	var u syscall.Utsname
	if err := syscall.Uname(&u); err != nil {
		return ""
	}
	out := make([]byte, 0, len(u.Release))
	for _, c := range u.Release {
		if c == 0 {
			break
		}
		out = append(out, byte(c))
	}
	return string(out)
}
