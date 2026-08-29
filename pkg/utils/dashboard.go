package utils

import (
	"io"
	"strings"
	"time"
)

var dashboardBroadcaster *EventBroadcaster

var DashboardWriter io.Writer = &DashboardLogWriter{}

func SetDashboardBroadcaster(eb *EventBroadcaster) { dashboardBroadcaster = eb }

func GetDashboardBroadcaster() *EventBroadcaster { return dashboardBroadcaster }

type DashboardLogWriter struct{}

func (w *DashboardLogWriter) Write(p []byte) (int, error) {
	if dashboardBroadcaster == nil {
		return len(p), nil
	}
	line := strings.TrimSpace(string(p))
	if line == "" {
		return len(p), nil
	}
	level, message := parseDashboardLogLine(line)
	data := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     level,
		"message":   message,
		"source":    "app",
	}
	dashboardBroadcaster.Broadcast("dashboard-logs", level, message, data)
	return len(p), nil
}

func parseDashboardLogLine(line string) (string, string) {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return "info", line
	}
	if len(parts[0]) == 8 && strings.Count(parts[0], ":") == 2 {
		switch strings.ToUpper(parts[1]) {
		case "DBG", "DEBUG":
			if len(parts) >= 3 {
				return "debug", parts[2]
			}
			return "debug", ""
		case "INF", "INFO":
			if len(parts) >= 3 {
				return "info", parts[2]
			}
			return "info", ""
		case "WRN", "WARN", "WARNING":
			if len(parts) >= 3 {
				return "warn", parts[2]
			}
			return "warn", ""
		case "ERR", "ERROR":
			if len(parts) >= 3 {
				return "error", parts[2]
			}
			return "error", ""
		case "FTL", "FATAL":
			if len(parts) >= 3 {
				return "error", parts[2]
			}
			return "error", ""
		}
		if len(parts) >= 3 {
			return "info", parts[2]
		}
	}
	upper := strings.ToUpper(line)
	switch {
	case strings.Contains(upper, "DEBUG"):
		return "debug", line
	case strings.Contains(upper, "WARN"):
		return "warn", line
	case strings.Contains(upper, "ERROR"):
		return "error", line
	case strings.Contains(upper, "FATAL"):
		return "error", line
	default:
		return "info", line
	}
}

var _ io.Writer = (*DashboardLogWriter)(nil)
