package api

import (
	"fmt"
	"net/http"
	"regexp"
	"sync/atomic"
)

// Counters exposed on /metrics in Prometheus text format.
var (
	metricFiscalizedTotal  atomic.Int64
	metricUnknownTotal     atomic.Int64
	metricFailedTotal      atomic.Int64
	metricExecutingTotal   atomic.Int64
	metricMQTTCmdTotal     atomic.Int64
	metricMQTTLatencyMsSum atomic.Int64 // milliseconds, divided by total for avg
)

// IncrFiscalOperationCounter increments the counter for the given operation state.
func IncrFiscalOperationCounter(state string) {
	switch state {
	case "FISCALIZED":
		metricFiscalizedTotal.Add(1)
	case "UNKNOWN":
		metricUnknownTotal.Add(1)
	case "FAILED":
		metricFailedTotal.Add(1)
	case "EXECUTING":
		metricExecutingTotal.Add(1)
	}
}

// RecordMQTTCommandLatency records a completed MQTT command round-trip.
func RecordMQTTCommandLatency(ms int64) {
	metricMQTTCmdTotal.Add(1)
	metricMQTTLatencyMsSum.Add(ms)
}

// metricsHandler returns counters in Prometheus text format.
func metricsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "# HELP fiscal_operations_total Total fiscal operations by state.\n")
	fmt.Fprintf(w, "# TYPE fiscal_operations_total counter\n")
	fmt.Fprintf(w, "fiscal_operations_total{state=\"FISCALIZED\"} %d\n", metricFiscalizedTotal.Load())
	fmt.Fprintf(w, "fiscal_operations_total{state=\"UNKNOWN\"} %d\n", metricUnknownTotal.Load())
	fmt.Fprintf(w, "fiscal_operations_total{state=\"FAILED\"} %d\n", metricFailedTotal.Load())
	fmt.Fprintf(w, "fiscal_operations_total{state=\"EXECUTING\"} %d\n", metricExecutingTotal.Load())
	fmt.Fprintf(w, "# HELP fiscal_unknown_total Total operations that ended in UNKNOWN state.\n")
	fmt.Fprintf(w, "# TYPE fiscal_unknown_total counter\n")
	fmt.Fprintf(w, "fiscal_unknown_total %d\n", metricUnknownTotal.Load())
	fmt.Fprintf(w, "# HELP mqtt_command_latency_seconds Average MQTT command round-trip latency in seconds.\n")
	fmt.Fprintf(w, "# TYPE mqtt_command_latency_seconds gauge\n")
	total := metricMQTTCmdTotal.Load()
	var avg float64
	if total > 0 {
		avg = float64(metricMQTTLatencyMsSum.Load()) / float64(total) / 1000.0
	}
	fmt.Fprintf(w, "mqtt_command_latency_seconds %g\n", avg)
}

// piiPattern matches Bulgarian EGN (10-digit personal IDs) and e-mail addresses.
var piiPattern = regexp.MustCompile(
	`\b\d{10}\b|[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`,
)

// MaskPII replaces EGN and e-mail patterns in a log line with [REDACTED].
func MaskPII(line string) string {
	return piiPattern.ReplaceAllString(line, "[REDACTED]")
}
