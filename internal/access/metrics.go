package access

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var MetricRejected = promauto.NewCounterVec(prometheus.CounterOpts{Name: "iolink_ingest_rejected_total", Help: "Rejected packets or fields, and persistence failures."}, []string{"reason"})
