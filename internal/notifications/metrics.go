package notifications

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MetricQueueDepth      = promauto.NewGauge(prometheus.GaugeOpts{Name: "iolink_notification_queue_depth", Help: "Notifications waiting for delivery."})
	MetricDeliverySeconds = promauto.NewHistogram(prometheus.HistogramOpts{Name: "iolink_notification_delivery_seconds", Help: "Notification delivery duration in seconds."})
)
