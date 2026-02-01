package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	OrdersTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "polygate_orders_total",
		Help: "The total number of orders processed",
	}, []string{"status", "side"})

	LatencyBucket = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "polygate_latency_bucket",
		Help:    "Request latency in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"endpoint"})

	RiskRejects = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "polygate_risk_rejects_total",
		Help: "Total risk engine rejections",
	}, []string{"reason"})
)
