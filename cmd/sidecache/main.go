package main

import (
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Trendyol/sidecache/pkg/cache"
	"github.com/Trendyol/sidecache/pkg/server"
	"go.uber.org/zap"
)

var counter = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: "sidecache",
		Name:      "cache_hit_counter",
		Help:      "This is my counter",
	})

var gauge = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Namespace: "sidecache",
		Name:      "cache_hit",
		Help:      "This is cache hit metric",
	})

func main() {
	logger, _ := zap.NewProduction()
	logger.Info("Side cache process started...")

	defer logger.Sync()

	prometheus.MustRegister(counter)

	couchbaseRepo := cache.NewCouchbaseRepository(logger)

	mainContainerPort := os.Getenv("MAIN_CONTAINER_PORT")
	mainContainerURL, err := url.Parse("http://127.0.0.1:" + mainContainerPort)
	if err != nil {
		logger.Error("Error occurred on url.Parse", zap.Error(err))
	}

	proxy := httputil.NewSingleHostReverseProxy(mainContainerURL)
	cacheServer := server.NewServer(couchbaseRepo, proxy, counter, logger)

	stopChan := make(chan int)
	cacheServer.Start(stopChan)
}
