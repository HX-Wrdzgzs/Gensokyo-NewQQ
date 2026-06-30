//go:build small

package webui

import (
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

// collectNativeSysInfo 使用各平台原生 API 采集系统信息，不依赖 gopsutil
func collectNativeSysInfo() gin.H {
	info := gin.H{
		"cpu_percent": getCPUPercent(),
		"memory": gin.H{
			"total":     getTotalMemory(),
			"available": getAvailableMemory(),
			"percent":   getMemoryPercent(),
		},
		"disk": gin.H{
			"total":   getTotalDisk(),
			"free":    getFreeDisk(),
			"percent": getDiskPercent(),
		},
		"boot_time": getBootTime(),
		"process": gin.H{
			"pid":         getPID(),
			"status":      "running",
			"memory_used": getProcessMemory(),
			"cpu_percent": getProcessCPU(),
			"start_time":  getStartTime(),
		},
	}
	return info
}

func handleSysInfo(c *gin.Context) {
	c.JSON(200, collectNativeSysInfo())
}

// ---------- 各平台需要实现的函数 ----------

// getCPUPercent 返回系统 CPU 使用率 (0-100)
func getCPUPercent() float64

// getTotalMemory 返回总物理内存 (bytes)
func getTotalMemory() uint64

// getAvailableMemory 返回可用物理内存 (bytes)
func getAvailableMemory() uint64

// getMemoryPercent 返回内存使用百分比 (0-100)
func getMemoryPercent() float64

// getTotalDisk 返回根分区总磁盘 (bytes)
func getTotalDisk() uint64

// getFreeDisk 返回根分区空闲磁盘 (bytes)
func getFreeDisk() uint64

// getDiskPercent 返回磁盘使用百分比 (0-100)
func getDiskPercent() float64

// getBootTime 返回系统启动时间戳
func getBootTime() int64

// getPID 返回当前进程 PID
func getPID() int

// getProcessMemory 返回当前进程占用内存 (bytes)
func getProcessMemory() uint64

// getProcessCPU 返回当前进程 CPU 使用率 (0-100)
func getProcessCPU() float64

// getStartTime 返回进程启动时间戳
func getStartTime() int64 {
	// 编译时注入，在 init 中记录
	return startTime
}

var startTime int64

func init() {
	startTime = time.Now().Unix()
}
