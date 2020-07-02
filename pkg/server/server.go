package server

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/Trendyol/sidecache/pkg/cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

const CacheHeaderKey = "tysidecarcachable"

type CacheServer struct {
	Repo           cache.CacheRepository
	Proxy          *httputil.ReverseProxy
	Counter        prometheus.Counter
	Logger         *zap.Logger
	CacheKeyPrefix string
}

func NewServer(repo cache.CacheRepository, proxy *httputil.ReverseProxy, counter prometheus.Counter, logger *zap.Logger) *CacheServer {
	return &CacheServer{
		Repo:           repo,
		Proxy:          proxy,
		Counter:        counter,
		Logger:         logger,
		CacheKeyPrefix: os.Getenv("CACHE_KEY_PREFIX"),
	}
}

func (server CacheServer) Start(stopChan chan int) {
	server.Proxy.ModifyResponse = func(r *http.Response) error {
		cacheHeaderValue := r.Header.Get(CacheHeaderKey)
		if cacheHeaderValue != "" {
			maxAgeInSecond := server.GetHeaderTTL(cacheHeaderValue)
			b, err := ioutil.ReadAll(r.Body)
			if err != nil {
				return err
			}

			go func(reqUrl *url.URL, data []byte) {
				hashedURL := server.HashURL(server.ReorderQueryString(reqUrl))
				server.Repo.SetKey(hashedURL, data, maxAgeInSecond)
			}(r.Request.URL, b)

			err = r.Body.Close()
			if err != nil {
				return err
			}

			body := ioutil.NopCloser(bytes.NewReader(b))
			r.Body = body
		}

		return nil
	}

	http.HandleFunc("/", server.CacheHandler)
	http.Handle("/metrics", promhttp.Handler())

	httpServer := &http.Server{Addr: ":9191"}

	go func() {
		server.Logger.Warn("Server closed: ", zap.Error(httpServer.ListenAndServe()))
	}()

	<-stopChan

	err := httpServer.Shutdown(context.Background())
	if err != nil {
		server.Logger.Error("shutdown hook error", zap.Error(err))
	}
}

func (server CacheServer) CacheHandler(w http.ResponseWriter, r *http.Request) {

	defer func() {
		if rec := recover(); rec != nil {
			var err error
			switch x := rec.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("Unknown panic")
			}

			server.Logger.Info("Recovered from panic", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}()

	hashedURL := server.HashURL(server.ReorderQueryString(r.URL))
	cachedData := server.CheckCache(hashedURL)

	if cachedData != nil {
		w.Header().Add("X-Cache-Response-For", r.URL.String())
		w.Header().Add("Content-Type", "application/json;charset=UTF-8")
		io.Copy(w, bytes.NewBuffer(cachedData))
		server.Counter.Inc()
	} else {
		server.Proxy.ServeHTTP(w, r)
	}
}

func (server CacheServer) GetHeaderTTL(cacheHeaderValue string) int {
	cacheValues := strings.Split(cacheHeaderValue, "=")
	var maxAgeInSecond = 0
	if len(cacheValues) > 1 {
		maxAgeInSecond, _ = strconv.Atoi(cacheValues[1])
	}
	return maxAgeInSecond
}

func (server CacheServer) HashURL(url string) string {
	hasher := md5.New()
	hasher.Write([]byte(server.CacheKeyPrefix + "/" + url))
	return hex.EncodeToString(hasher.Sum(nil))
}

func (server CacheServer) CheckCache(url string) []byte {
	return server.Repo.Get(url)
}

func (server CacheServer) ReorderQueryString(url *url.URL) string {
	return url.Path + "?" + url.Query().Encode()
}
