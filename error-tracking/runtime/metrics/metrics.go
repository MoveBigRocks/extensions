package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ErrorsIngested = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mbr_error_tracking_errors_ingested_total",
		Help: "Total errors ingested by the error-tracking extension.",
	}, []string{"workspace", "project"})
	IssuesChanged = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mbr_error_tracking_issues_changed_total",
		Help: "Total issue lifecycle changes processed by the error-tracking extension.",
	}, []string{"workspace", "change"})
)
