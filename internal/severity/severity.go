package severity

import "fmt"

const (
	CPUHigh            = 80
	WaitTimeWarning    = 30.0
	WaitTimeCritical   = 120.0
	QueryReadsHigh     = 100000
	PLELow             = 300
)

type Result struct {
	Severity string
	Reasons  []string
}

func Calculate(procStats, activeQueries, serverResources []map[string]interface{}) *Result {
	severity := "GREEN"
	var reasons []string

	if activeQueries != nil {
		for _, row := range activeQueries {
			waitTime := getFloat64(row, "wait_time_seconds")
			if waitTime > WaitTimeCritical {
				severity = "RED"
				sessionID := getInt64(row, "session_id")
				reasons = append(reasons, fmt.Sprintf("Query waiting %.1fs (session %d)", waitTime, sessionID))
			} else if waitTime > WaitTimeWarning {
				if severity != "RED" {
					severity = "YELLOW"
				}
				reasons = append(reasons, fmt.Sprintf("Query waiting %.1fs", waitTime))
			}

			blockingSessionID := getInt64(row, "blocking_session_id")
			if blockingSessionID > 0 {
				if severity == "GREEN" {
					severity = "YELLOW"
				}
				sessionID := getInt64(row, "session_id")
				reasons = append(reasons, fmt.Sprintf("Blocking detected (session %d)", sessionID))
			}

			logicalReads := getInt64(row, "logical_reads")
			if logicalReads > QueryReadsHigh {
				if severity == "GREEN" {
					severity = "YELLOW"
				}
				reasons = append(reasons, fmt.Sprintf("High reads: %d", logicalReads))
			}
		}
	}

	if serverResources != nil && len(serverResources) > 0 {
		row := serverResources[0]

		cpuPercent := getInt64(row, "cpu_percent")
		if cpuPercent > 0 && cpuPercent > CPUHigh {
			if severity == "GREEN" {
				severity = "YELLOW"
			}
			reasons = append(reasons, fmt.Sprintf("CPU at %d%%", cpuPercent))
		}

		ple := getInt64(row, "page_life_expectancy")
		if ple > 0 && ple < PLELow {
			if severity == "GREEN" {
				severity = "YELLOW"
			}
			reasons = append(reasons, fmt.Sprintf("Low PLE: %d", ple))
		}

		blockingCount := getInt64(row, "blocking_sessions_count")
		if blockingCount > 0 {
			if severity == "GREEN" {
				severity = "YELLOW"
			}
			reasons = append(reasons, fmt.Sprintf("Blocking sessions: %d", blockingCount))
		}
	}

	if activeQueries == nil || serverResources == nil {
		severity = "ERROR"
		reasons = append(reasons, "Failed to collect some metrics")
	}

	return &Result{
		Severity: severity,
		Reasons:  reasons,
	}
}

func getFloat64(row map[string]interface{}, key string) float64 {
	val, ok := row[key]
	if !ok {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

func getInt64(row map[string]interface{}, key string) int64 {
	val, ok := row[key]
	if !ok {
		return 0
	}
	switch v := val.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	default:
		return 0
	}
}
