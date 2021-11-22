package main

import (
	"github.com/zeriontech/sidecache/pkg/cache"
	"github.com/zeriontech/sidecache/pkg/server"
	"go.uber.org/zap"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

var version string

func main() {
	logger, _ := zap.NewProduction()
	logger.Info("Side cache process started...", zap.String("version", version))

	defer logger.Sync()
	cacheRepo, err := cache.NewRedisRepository(logger)
	if err != nil {
		for {
			logger.Warn("Redis is not connected, retrying...")
			if repo, err := cache.NewRedisRepository(logger); err == nil {
				cacheRepo = repo
				break
			}
			time.Sleep(3 * time.Second)
		}
	}
	logger.Info("Redis is connected.")

	mainContainerPort := os.Getenv("MAIN_CONTAINER_PORT")
	logger.Info("Main container port", zap.String("port", mainContainerPort))
	mainContainerURL, err := url.Parse("http://127.0.0.1:" + mainContainerPort)
	if err != nil {
		logger.Error("Error occurred on url.Parse", zap.Error(err))
	}

	prom := server.NewPrometheusClient()

	server.BuildInfo(version)

	proxy := httputil.NewSingleHostReverseProxy(mainContainerURL)

	cacheServer := server.NewServer(cacheRepo, proxy, prom, logger)
	logger.Info("Cache key prefix", zap.String("prefix", cacheServer.CacheKeyPrefix))

	stopChan := make(chan int)
	cacheServer.Start(stopChan)
}
