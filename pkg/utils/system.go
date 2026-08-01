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
)

var (
	runtimeMemStats  atomic.Pointer[runtime.MemStats]
	statsMutex       sync.Mutex
	runtimeStats     atomic.Bool
	memSelfInterval  atomic.Int64 // nanoseconds
	memSelfLastFetch atomic.Int64 // UnixNano
	memSelfValue     atomic.Uint64

	routineLastFetch    atomic.Int64 // UnixNano
	routineInterval     atomic.Int64 // nanoseconds
	routineFirstFetched atomic.Bool
	routineValue        atomic.Int32
)

// getRuntimeStats gathers runtime.
func getRuntimeStats() runtime.MemStats {
	if !runtimeStats.Load() {
		statsMutex.Lock()
		if !runtimeStats.Load() { // double-check
			staged := runtime.MemStats{}
			runtime.ReadMemStats(&staged)
			runtimeMemStats.Store(&staged)
			memSelfInterval.Store(int64(5 * time.Second))
			memSelfValue.Store(0)
			routineValue.Store(0)
			routineInterval.Store(int64(5 * time.Second))
			runtimeStats.Store(true)
		}
		statsMutex.Unlock()
	}
	// Atomically load pointer — loads on Ptr-typed atomics are already fully
	// synchronised, so copying the dereferenced struct is race-free without a
	// spin-loop (the old double-Load pattern never converged here, as the
	// background writer only swaps the pointer every 5 s).
	p := runtimeMemStats.Load()
	if p == nil {
		return runtime.MemStats{}
	}
	return *p
}

// GetMemSelf gathers stackyrd memory usage.
func GetMemSelf() uint64 {
	_ = getRuntimeStats() // ensure background stats goroutine is running

	last := memSelfLastFetch.Load()
	now := time.Now()
	if last == 0 || now.UnixNano()-last >= memSelfInterval.Load() {
		if p := runtimeMemStats.Load(); p != nil {
			memSelfValue.Store(p.Sys / 1024 / 1024)
		}
		memSelfLastFetch.Store(now.UnixNano())
	}
	return memSelfValue.Load()
}

func GetRoutine() int {
	if !routineFirstFetched.Load() {
		routineFirstFetched.Store(true)
		routineValue.Store(int32(runtime.NumGoroutine()))
		return int(routineValue.Load())
	}

	last := routineLastFetch.Load()
	now := time.Now()
	if last == 0 || now.UnixNano()-last >= routineInterval.Load() {
		routineLastFetch.Store(now.UnixNano())
		routineValue.Store(int32(runtime.NumGoroutine()))
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

func resetTerminal() {
	os.Stdout.WriteString("\033[?1049l\033[0m\033[H")
}

// ClearScreen clears the terminal screen (cross-platform)
// Also resets terminal state: restores main screen buffer, cursor, and text attributes.
func ClearScreen() {
	resetTerminal()
	os.Stdout.WriteString("\033[2J\033[3J")

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "cls")
	default:
		cmd = exec.Command("clear")
	}

	cmd.Stdout = os.Stdout
	_ = cmd.Run()
}

// CheckPortAvailability checks if the required ports are available before starting the application
func CheckPortAvailability(serverPort string) error {
	// Check server port
	if err := CheckPort(serverPort); err != nil {
		return fmt.Errorf("server port %s is already in use: %w", serverPort, err)
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
	defer func() { _ = listener.Close() }()
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
