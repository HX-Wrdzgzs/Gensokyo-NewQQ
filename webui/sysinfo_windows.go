//go:build small && windows

package webui

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemTimes    = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetNativeSystemInfo = kernel32.NewProc("GetNativeSystemInfo")
	procGetProcessTimes   = kernel32.NewProc("GetProcessTimes")
	// psapi.dll for GetProcessMemoryInfo
	psapi                      = syscall.NewLazyDLL("psapi.dll")
	procGetProcessMemoryInfo   = psapi.NewProc("GetProcessMemoryInfo")
	getCurrentProcess = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentProcess")
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
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uint64
	WorkingSetSize             uint64
	QuotaPeakPagedPoolUsage    uint64
	QuotaPagedPoolUsage        uint64
	QuotaPeakNonPagedPoolUsage uint64
	QuotaNonPagedPoolUsage     uint64
	PagefileUsage              uint64
	PeakPagefileUsage          uint64
}

var (
	prevIdleTime   uint64
	prevKernelTime uint64
	prevUserTime   uint64
	prevCPUTime    time.Time
	cpuInited      bool
)

func getCPUPercent() float64 {
	var idle, kernel, user syscall.Filetime
	ret, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if ret == 0 {
		return 0
	}
	now := time.Now()
	idleTime := uint64(idle.HighDateTime)<<32 | uint64(idle.LowDateTime)
	kernelTime := uint64(kernel.HighDateTime)<<32 | uint64(kernel.LowDateTime)
	userTime := uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime)

	if !cpuInited {
		prevIdleTime = idleTime
		prevKernelTime = kernelTime
		prevUserTime = userTime
		prevCPUTime = now
		cpuInited = true
		return 0
	}

	idleDelta := idleTime - prevIdleTime
	kernelDelta := kernelTime - prevKernelTime
	userDelta := userTime - prevUserTime
	totalDelta := kernelDelta + userDelta
	timeDelta := uint64(now.Sub(prevCPUTime).Nanoseconds()) * 100 // 100ns units

	prevIdleTime = idleTime
	prevKernelTime = kernelTime
	prevUserTime = userTime
	prevCPUTime = now

	if totalDelta == 0 || timeDelta == 0 {
		return 0
	}
	usage := float64(totalDelta-idleDelta) / float64(totalDelta) * 100
	if usage < 0 {
		return 0
	}
	return usage
}

func getTotalMemory() uint64 {
	var ms memoryStatusEx
	ms.Length = uint32(unsafe.Sizeof(ms))
	ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	if ret == 0 {
		return 0
	}
	return ms.TotalPhys
}

func getAvailableMemory() uint64 {
	var ms memoryStatusEx
	ms.Length = uint32(unsafe.Sizeof(ms))
	ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	if ret == 0 {
		return 0
	}
	return ms.AvailPhys
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
	freeBytesAvailable := int64(0)
	totalNumberOfBytes := int64(0)
	totalNumberOfFreeBytes := int64(0)
	err := syscall.GetDiskFreeSpaceEx(
		syscall.StringToUTF16Ptr("."),
		&freeBytesAvailable,
		&totalNumberOfBytes,
		&totalNumberOfFreeBytes,
	)
	if err != nil {
		return 0
	}
	return uint64(totalNumberOfBytes)
}

func getFreeDisk() uint64 {
	freeBytesAvailable := int64(0)
	totalNumberOfBytes := int64(0)
	totalNumberOfFreeBytes := int64(0)
	err := syscall.GetDiskFreeSpaceEx(
		syscall.StringToUTF16Ptr("."),
		&freeBytesAvailable,
		&totalNumberOfBytes,
		&totalNumberOfFreeBytes,
	)
	if err != nil {
		return 0
	}
	return uint64(totalNumberOfFreeBytes)
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
	// Windows: GetTickCount64 获取运行毫秒数
	proc := kernel32.NewProc("GetTickCount64")
	ms, _, _ := proc.Call()
	if ms == 0 {
		return time.Now().Unix()
	}
	return time.Now().Unix() - int64(ms/1000)
}

func getPID() int {
	return syscall.Getpid()
}

func getProcessMemory() uint64 {
	hProc, _, _ := getCurrentProcess.Call()
	if hProc == 0 {
		return 0
	}
	var pmc processMemoryCounters
	pmc.CB = uint32(unsafe.Sizeof(pmc))
	ret, _, _ := procGetProcessMemoryInfo.Call(hProc, uintptr(unsafe.Pointer(&pmc)), uintptr(pmc.CB))
	if ret == 0 {
		return 0
	}
	return pmc.WorkingSetSize
}

func getProcessCPU() float64 {
	var creationTime, exitTime, kernelTime, userTime syscall.Filetime
	hProc, _, _ := getCurrentProcess.Call()
	if hProc == 0 {
		return 0
	}
	ret, _, _ := procGetProcessTimes.Call(
		hProc,
		uintptr(unsafe.Pointer(&creationTime)),
		uintptr(unsafe.Pointer(&exitTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)
	if ret == 0 {
		return 0
	}
	// 简化处理：返回 0（进程 CPU 较复杂，需要多核校准）
	return 0
}
