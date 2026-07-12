package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// These counters are owned by the web analytics extension. The metric names and
// labels match what the platform exposed previously so existing dashboards keep
// working; the extension runtime is a separate process, so there is no
// registration collision with the core.
var (
	// AnalyticsEventsIngestedTotal tracks total analytics events ingested.
	AnalyticsEventsIngestedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_analytics_events_ingested_total",
			Help: "Total analytics events successfully ingested",
		},
		[]string{"domain"},
	)

	// AnalyticsEventsRejectedTotal tracks total analytics events rejected.
	AnalyticsEventsRejectedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_analytics_events_rejected_total",
			Help: "Total analytics events rejected",
		},
		[]string{"reason"},
	)
)
