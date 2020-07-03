package main

import (
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/Trendyol/sidecache/pkg/cache"
	"github.com/Trendyol/sidecache/pkg/server"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	logger.Info("Side cache process started...")

	defer logger.Sync()

	couchbaseRepo := cache.NewCouchbaseRepository(logger)

	mainContainerPort := os.Getenv("MAIN_CONTAINER_PORT")
	logger.Info("Main container port", zap.String("port", mainContainerPort))
	mainContainerURL, err := url.Parse("http://127.0.0.1:" + mainContainerPort)
	if err != nil {
		logger.Error("Error occurred on url.Parse", zap.Error(err))
	}

	prom := server.NewPrometheusClient()
	proxy := httputil.NewSingleHostReverseProxy(mainContainerURL)
	cacheServer := server.NewServer(couchbaseRepo, proxy, prom, logger)
	logger.Info("Cache key prefix", zap.String("prefix", cacheServer.CacheKeyPrefix))

	stopChan := make(chan int)
	cacheServer.Start(stopChan)
}
