package utils

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

var (
	// GetMemSelf — atomic to avoid data-race on concurrent reads vs background writes
	runtimeMemStats atomic.Pointer[runtime.MemStats]
	statsMutex       sync.Mutex         // protects writes via GetRuntimeStats
	runtimeStats     bool
	memSelfInterval  time.Duration
	memSelfLastFetch time.Time
	memSelfValue     atomic.Uint64

	// GetRoutine
	routineLastFetch      time.Time
	routineInterval       time.Duration
	routineFirstFetched   bool
	routineValue          atomic.Int32
)

// GetSystemStats gathers CPU and Memory usage.
func GetSystemStats() (map[string]interface{}, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %w", err)
	}

	c, err := cpu.Percent(100*time.Millisecond, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get cpu stats: %w", err)
	}

	stats := map[string]interface{}{
		"cpu_percent":         c[0],
		"memory_total_mb":     v.Total / 1024 / 1024,
		"memory_used_mb":      v.Used / 1024 / 1024,
		"memory_used_percent": v.UsedPercent,
		"go_routines":         runtime.NumGoroutine(),
		"os":                  runtime.GOOS,
		"arch":                runtime.GOARCH,
	}

	return stats, nil
}

// GetProcessInfo gathers info about the current process.
func GetProcessInfo() (map[string]interface{}, error) {
	pid := int32(os.Getpid())
	p, err := process.NewProcess(pid)
	if err != nil {
		return nil, fmt.Errorf("failed to get process stats: %w", err)
	}

	memInfo, err := p.MemoryInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get process memory stats: %w", err)
	}

	cpuPercent, err := p.CPUPercent()
	if err != nil {
		return nil, fmt.Errorf("failed to get process cpu stats: %w", err)
	}

	info := map[string]interface{}{
		"pid":           pid,
		"memory_rss_mb": memInfo.RSS / 1024 / 1024,
		"cpu_percent":   cpuPercent,
	}

	return info, nil
}

// GetDiskUsage gathers disk usage info for root path.
func GetDiskUsage() (map[string]interface{}, error) {
	parts, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	// Just check the first partition or root usually
	var usage *disk.UsageStat
	if runtime.GOOS == "windows" {
		usage, err = disk.Usage("C:\\")
	} else {
		usage, err = disk.Usage("/")
	}

	if err != nil {
		// Fallback to first partition if C:\ or / fails?
		if len(parts) > 0 {
			usage, err = disk.Usage(parts[0].Mountpoint)
		}
	}

	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"path":         usage.Path,
		"total_gb":     usage.Total / 1024 / 1024 / 1024,
		"used_gb":      usage.Used / 1024 / 1024 / 1024,
		"used_percent": usage.UsedPercent,
	}, nil
}

// GetRuntimeStats gathers runtime.
func GetRuntimeStats() runtime.MemStats {
	if !runtimeStats {
		statsMutex.Lock()
		defer statsMutex.Unlock()
		if !runtimeStats { // double-check
			staged := runtime.MemStats{}
			runtime.ReadMemStats(&staged)
			runtimeMemStats.Store(&staged)
			memSelfInterval = 5 * time.Second
			memSelfValue.Store(0)
			routineValue.Store(0)
			routineInterval = 5 * time.Second
			runtimeStats = true
		}
	}
	// Atomically load pointer — loads on Ptr-typed atomics are already fully
	// synchronised, so copying the dereferenced struct is race-free without a
	// spin-loop (the old double-Load pattern never converged here, as the
	// background writer only swaps the pointer every 5 s).
	p := runtimeMemStats.Load()
	if p == nil {
		return runtime.MemStats{}
	}
	_ = *p  // force dereference to prove no escape (p is already a pointer copy)
	return *p
}

// GetMemSelf gathers stackyrd memory usage.
func GetMemSelf() uint64 {
	_ = GetRuntimeStats() // ensure background stats goroutine is running

	if memSelfLastFetch.IsZero() || time.Since(memSelfLastFetch) >= memSelfInterval {
		alloc := runtimeMemStats.Load().Sys
		memSelfValue.Store(alloc / 1024 / 1024)
		memSelfLastFetch = time.Now()
	}
	return memSelfValue.Load()
}

func GetRoutine() int {
	if !routineFirstFetched {
		routineInterval = 5 * time.Second
		routineFirstFetched = true
	} else {
		if routineLastFetch.IsZero() || time.Since(routineLastFetch) >= routineInterval {
			routineLastFetch = time.Now()
			routineValue.Store(int32(runtime.NumGoroutine()))
		}
	}
	return int(routineValue.Load())
}

// GetNetworkInfo gathers hostname and IP.
func GetNetworkInfo() (map[string]string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	ip := "unknown"
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ip = ipnet.IP.String()
					break
				}
			}
		}
	}

	return map[string]string{
		"hostname": hostname,
		"ip":       ip,
	}, nil
}

// ClearScreen clears the terminal screen (cross-platform)
func ClearScreen() {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// Windows: use cmd /c cls
		cmd = exec.Command("cmd", "/c", "cls")
	default:
		// Linux, macOS, and others: use clear command
		cmd = exec.Command("clear")
	}

	cmd.Stdout = os.Stdout
	cmd.Run()
}

// CheckPortAvailability checks if the required ports are available before starting the application
func CheckPortAvailability(serverPort string) error {
	// Check server port
	if err := CheckPort(serverPort); err != nil {
		return fmt.Errorf("server port %s is already in use: %v \n", serverPort, err)
	}

	return nil
}

// CheckPort checks if a specific port is available
func CheckPort(port string) error {
	// Try to listen on the port
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	defer listener.Close()
	return nil
}

// ShutdownChan is a global shutdown channel for TUI communication
var ShutdownChan = make(chan struct{})

// TriggerShutdown sends a shutdown signal to the main thread
func TriggerShutdown() {
	select {
	case ShutdownChan <- struct{}{}:
		// Successfully sent shutdown signal
	default:
		// Channel is full or closed, ignore
	}
}
