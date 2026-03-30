package infrastructure

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"stackyard/config"
	"stackyard/pkg/logger"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

type SystemManager struct{}

// Name returns the display name of the component
func (s *SystemManager) Name() string {
	return "System Monitor"
}

func NewSystemManager() *SystemManager {
	return &SystemManager{}
}

func (s *SystemManager) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// Memory
	vm, err := mem.VirtualMemory()
	if err == nil {
		stats["memory"] = map[string]interface{}{
			"total_gb":     fmt.Sprintf("%.1f", float64(vm.Total)/1024/1024/1024),
			"used_gb":      fmt.Sprintf("%.1f", float64(vm.Used)/1024/1024/1024),
			"used_percent": fmt.Sprintf("%.1f", vm.UsedPercent),
			"free_gb":      fmt.Sprintf("%.1f", float64(vm.Free)/1024/1024/1024),
		}
	}

	// CPU
	percent, err := cpu.Percent(time.Second*0, false) // 0 wait for instant check
	if err == nil && len(percent) > 0 {
		stats["cpu"] = map[string]interface{}{
			"usage_percent": fmt.Sprintf("%.1f", percent[0]),
			"cores":         len(percent), // simple count if percpu=true, but here we used false so it's total
		}
		// Get cores count separate
		cores, _ := cpu.Counts(true)
		stats["cpu"].(map[string]interface{})["cores"] = cores
	}

	// Disk
	parts, err := disk.Partitions(false)
	if err == nil && len(parts) > 0 {
		// Just check C: or root
		usage, err := disk.Usage(parts[0].Mountpoint)
		if err == nil {
			stats["disk"] = map[string]interface{}{
				"path":         usage.Path,
				"total_gb":     fmt.Sprintf("%.1f", float64(usage.Total)/1024/1024/1024),
				"used_gb":      fmt.Sprintf("%.1f", float64(usage.Used)/1024/1024/1024),
				"used_percent": fmt.Sprintf("%.1f", usage.UsedPercent),
			}
		}
	}

	return stats
}

// GetHostInfo returns static system information
func (s *SystemManager) GetHostInfo() map[string]string {
	hostname, _ := os.Hostname()

	// Simple outbound IP detection
	ip := "127.0.0.1"
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		ip = localAddr.IP.String()
		conn.Close()
	}

	return map[string]string{
		"hostname": hostname,
		"ip":       ip,
		"os":       runtime.GOOS,
		"arch":     runtime.GOARCH,
	}
}

func init() {
	RegisterComponent("system", func(cfg *config.Config, l *logger.Logger) (InfrastructureComponent, error) {
		return NewSystemManager(), nil
	})
}

// Close closes the system monitor (no-op for system monitor)
func (s *SystemManager) Close() error {
	return nil
}

// GetStatus returns the current status of the system monitor
func (s *SystemManager) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"active": true,
	}
}
