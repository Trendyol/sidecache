package server

import "github.com/prometheus/client_golang/prometheus"

var gauge = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Namespace: "sidecache",
		Name:      "cache_hit",
		Help:      "This is cache hit metric",
	})

var cacheHitCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: "sidecache",
		Name:      "cache_hit_counter",
		Help:      "Cache hit count",
	})

var totalRequestCounter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: "sidecache",
		Name:      "all_request_hit_counter",
		Help:      "All request hit counter",
	})

type Prometheus struct {
	CacheHitCounter     prometheus.Counter
	TotalRequestCounter prometheus.Counter
}

func NewPrometheusClient() *Prometheus {
	prometheus.MustRegister(cacheHitCounter, totalRequestCounter)

	return &Prometheus{TotalRequestCounter: totalRequestCounter, CacheHitCounter: cacheHitCounter}
}
