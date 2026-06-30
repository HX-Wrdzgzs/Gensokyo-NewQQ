//go:build small && linux

package webui

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	prevCPUStats   cpuStats
	cpuInited      bool
	procStartTime  = time.Now()
)

type cpuStats struct {
	user    uint64
	nice    uint64
	system  uint64
	idle    uint64
	iowait  uint64
	irq     uint64
	softirq uint64
	steal   uint64
}

func readProcFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func parseUint64(s string) uint64 {
	v, _ := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	return v
}

func getCPUPercent() float64 {
	data, err := readProcFile("/proc/stat")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(data, "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			return 0
		}
		var cur cpuStats
		cur.user = parseUint64(fields[1])
		cur.nice = parseUint64(fields[2])
		cur.system = parseUint64(fields[3])
		cur.idle = parseUint64(fields[4])
		cur.iowait = parseUint64(fields[5])
		cur.irq = parseUint64(fields[6])
		cur.softirq = parseUint64(fields[7])

		if !cpuInited {
			prevCPUStats = cur
			cpuInited = true
			return 0
		}

		prevIdle := prevCPUStats.idle + prevCPUStats.iowait
		idle := cur.idle + cur.iowait
		prevTotal := prevCPUStats.user + prevCPUStats.nice + prevCPUStats.system +
			prevCPUStats.idle + prevCPUStats.iowait + prevCPUStats.irq +
			prevCPUStats.softirq + prevCPUStats.steal
		total := cur.user + cur.nice + cur.system + cur.idle + cur.iowait +
			cur.irq + cur.softirq + cur.steal

		totalDelta := total - prevTotal
		idleDelta := idle - prevIdle

		prevCPUStats = cur

		if totalDelta == 0 {
			return 0
		}
		return float64(totalDelta-idleDelta) / float64(totalDelta) * 100
	}
	return 0
}

func getTotalMemory() uint64 {
	data, err := readProcFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return parseUint64(fields[1]) * 1024 // kB → bytes
			}
		}
	}
	return 0
}

func getAvailableMemory() uint64 {
	data, err := readProcFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return parseUint64(fields[1]) * 1024
			}
		}
	}
	// fallback: MemFree + Buffers + Cached
	var memFree, buffers, cached uint64
	for _, line := range strings.Split(data, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		switch {
		case strings.HasPrefix(line, "MemFree:"):
			memFree = parseUint64(f[1]) * 1024
		case strings.HasPrefix(line, "Buffers:"):
			buffers = parseUint64(f[1]) * 1024
		case strings.HasPrefix(line, "Cached:"):
			cached = parseUint64(f[1]) * 1024
		}
	}
	if memFree+buffers+cached > 0 {
		return memFree + buffers + cached
	}
	return 0
}

func getMemoryPercent() float64 {
	total := getTotalMemory()
	avail := getAvailableMemory()
	if total == 0 {
		return 0
	}
	return float64(total-avail) / float64(total) * 100
}

func getTotalDisk() uint64 {
	var stat syscall.Statfs_t
	err := syscall.Statfs("/", &stat)
	if err != nil {
		return 0
	}
	return stat.Blocks * uint64(stat.Bsize)
}

func getFreeDisk() uint64 {
	var stat syscall.Statfs_t
	err := syscall.Statfs("/", &stat)
	if err != nil {
		return 0
	}
	return stat.Bfree * uint64(stat.Bsize)
}

func getDiskPercent() float64 {
	total := getTotalDisk()
	free := getFreeDisk()
	if total == 0 {
		return 0
	}
	return float64(total-free) / float64(total) * 100
}

func getBootTime() int64 {
	data, err := readProcFile("/proc/stat")
	if err != nil {
		return time.Now().Unix()
	}
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "btime ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return int64(parseUint64(fields[1]))
			}
		}
	}
	return time.Now().Unix()
}

func getPID() int {
	return os.Getpid()
}

func getProcessMemory() uint64 {
	data, err := readProcFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return parseUint64(fields[1]) * 1024
			}
		}
	}
	return 0
}

func getProcessCPU() float64 {
	// 简化：读取 /proc/self/stat 计算进程 CPU
	data, err := readProcFile("/proc/self/stat")
	if err != nil {
		return 0
	}
	fields := strings.Fields(data)
	if len(fields) < 15 {
		return 0
	}
	// field[13] utime, field[14] stime (clock ticks)
	utime := parseUint64(fields[13])
	stime := parseUint64(fields[14])
	totalTicks := utime + stime

	// 获取时钟频率
	clkTck, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0
	}
	_ = clkTck
	// 简化：用运行时间和总 CPU 时间估算
	uptime := time.Since(procStartTime).Seconds()
	if uptime <= 0 {
		return 0
	}
	// 假设 clock ticks = 100 (大多数 Linux 默认)
	return float64(totalTicks) / 100 / uptime * 100
}
