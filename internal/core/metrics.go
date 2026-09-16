package core

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics for the /metrics endpoint. All increments happen on the
// ingest path; keep them cheap (atomic counters/gauges).
var (
	MetricTelemetryTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "iolink_telemetry_total",
		Help: "Accepted telemetry reports since start.",
	})
	MetricAlarmsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "iolink_alarms_total",
		Help: "Alarms raised since start.",
	})
	MetricDevicesOnline = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "iolink_devices_online",
		Help: "Currently online devices (per connect/disconnect events).",
	})
	MetricNotificationsSent = promauto.NewCounter(prometheus.CounterOpts{
		Name: "iolink_notifications_total",
		Help: "Alarm notifications dispatched to notifiers.",
	})
)
