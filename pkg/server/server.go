package server

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/amyangfei/redlock-go/v2/redlock"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zeriontech/sidecache/pkg/cache"
	"go.uber.org/zap"
)

const CacheHeaderEnabledKey = "sidecache-headers-enabled"
const applicationDefaultPort = ":9191"

type CacheServer struct {
	Repo           cache.Repository
	LockMgr        *redlock.RedLock
	Proxy          *httputil.ReverseProxy
	Prometheus     *Prometheus
	Logger         *zap.Logger
	CacheKeyPrefix string
}

type CacheData struct {
	Body       []byte
	Headers    map[string]string
	StatusCode int
}

func NewServer(repo cache.Repository, lockMgr *redlock.RedLock, proxy *httputil.ReverseProxy, prom *Prometheus, logger *zap.Logger) *CacheServer {
	return &CacheServer{
		Repo:           repo,
		LockMgr:        lockMgr,
		Proxy:          proxy,
		Prometheus:     prom,
		Logger:         logger,
		CacheKeyPrefix: os.Getenv("CACHE_KEY_PREFIX"),
	}
}

func (server CacheServer) Start(stopChan chan int) {
	server.Proxy.ModifyResponse = func(r *http.Response) error {
		if r.StatusCode >= 500 {
			return nil
		}

		cacheHeadersEnabled := r.Header.Get(CacheHeaderEnabledKey)
		maxAgeInSecond, err := time.ParseDuration(os.Getenv("CACHE_TTL"))

		if err != nil {
			server.Logger.Error("invalid cache TTL", zap.Error(err))
			return nil
		}

		r.Header.Del("Content-Length") // https://github.com/golang/go/issues/14975
		b, err := ioutil.ReadAll(r.Body)

		if err != nil {
			server.Logger.Error("Error while reading response body", zap.Error(err))
			return err
		}

		go func(reqUrl *url.URL, data []byte, statusCode int, ttl time.Duration, cacheHeadersEnabled string) {
			hashedURL := server.HashURL(server.ReorderQueryString(reqUrl))
			cacheData := CacheData{Body: data, StatusCode: statusCode}

			if cacheHeadersEnabled == "true" {
				headers := make(map[string]string)
				for h, v := range r.Header {
					headers[h] = strings.Join(v, ";")
				}
				cacheData.Headers = headers
			}

			cacheDataBytes, _ := json.Marshal(cacheData)
			server.Repo.SetKey(hashedURL, cacheDataBytes, ttl)
		}(r.Request.URL, b, r.StatusCode, maxAgeInSecond, cacheHeadersEnabled)

		err = r.Body.Close()
		if err != nil {
			server.Logger.Error("Error while closing response body", zap.Error(err))
			return err
		}

		r.Body = ioutil.NopCloser(bytes.NewReader(b))

		return nil
	}

	http.HandleFunc("/", server.CacheHandler)
	http.Handle("/metrics", promhttp.Handler())

	port := determinatePort()
	httpServer := &http.Server{Addr: port}
	server.Logger.Info("SideCache process started port: ", zap.String("port", port))

	go func() {
		server.Logger.Warn("Server closed: ", zap.Error(httpServer.ListenAndServe()))
	}()

	<-stopChan

	err := httpServer.Shutdown(context.Background())
	if err != nil {
		server.Logger.Error("shutdown hook error", zap.Error(err))
	}
}

func determinatePort() string {
	customPort := os.Getenv("SIDE_CACHE_PORT")
	if customPort == "" {
		return applicationDefaultPort

	}
	return ":" + customPort
}

func (server CacheServer) CacheHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.TODO()
	server.Prometheus.TotalRequestCounter.Inc()

	defer func() {
		if rec := recover(); rec != nil {
			var err error
			switch x := rec.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("unknown panic")
			}

			server.Logger.Info("Recovered from panic", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}()

	key := server.HashURL(server.ReorderQueryString(r.URL)) + "-lock"
	lockTTL := 15 * time.Second  // todo: configure TTL
	_, err := server.LockMgr.Lock(ctx, key, lockTTL)
	if err != nil {
		// some other goroutine is already working on the same request
		acquired := make(chan string, 1)

		go func(done chan string) {
			for i := 0; i < 7; i++ {
				// back-offs: 10ms - 50ms - 100ms - 500ms - 1s - 5s - 10s
				multiplier := 1
				if i % 2 != 0 {
					multiplier = 5
				}
				backoff := time.Duration(multiplier * int(math.Pow(10, float64(i / 2 + 1)))) * time.Millisecond
				server.Logger.Info("wait lock", zap.Duration("backoff", backoff), zap.String("url", r.URL.String()))
				time.Sleep(backoff)

				if _, err := server.LockMgr.Lock(ctx, key, lockTTL); err == nil {
					// lock is acquired
					acquired <- "success"
					return
				}
			}
			acquired <- "fail"
		}(acquired)

		// wait for the lock
		if result := <-acquired; result != "success" {
			server.Logger.Error("failed to acquire the lock")
		}
	}

	// lock is acquired
	serve(server, w, r)

	// unlock the lock
	if err := server.LockMgr.UnLock(ctx, key); err != nil {
		server.Logger.Error("Could not unlock the lock", zap.Error(err))
	}
}

func serve(server CacheServer, w http.ResponseWriter, r *http.Request) {
	hashedURL := server.HashURL(server.ReorderQueryString(r.URL))
	cachedDataBytes := server.CheckCache(hashedURL)

	server.Logger.Info("serve request", zap.String("url", r.URL.String()), zap.Bool("cached", cachedDataBytes != nil))

	if cachedDataBytes != nil {
		serveFromCache(cachedDataBytes, server, w, r)
	} else {
		server.Proxy.ServeHTTP(w, r)
	}
}

func serveFromCache(cachedDataBytes []byte, server CacheServer, w http.ResponseWriter, r *http.Request) {
	w.Header().Add("X-Cache-Response-For", r.URL.String())
	w.Header().Add("Content-Type", "application/json;charset=UTF-8")  // todo get from cache?

	var cachedData CacheData
	err := json.Unmarshal(cachedDataBytes, &cachedData)
	if err != nil {
		server.Logger.Error("Can not unmarshal cached data", zap.Error(err))
		return
	}

	writeHeaders(w, cachedData.Headers)
	w.WriteHeader(cachedData.StatusCode)

	if _, err := io.Copy(w, bytes.NewReader(cachedData.Body)); err != nil {
		server.Logger.Error("IO error", zap.Error(err))
		return
	}

	server.Prometheus.CacheHitCounter.Inc()
}

func writeHeaders(w http.ResponseWriter, headers map[string]string) {
	if headers != nil {
		for h, v := range headers {
			w.Header().Set(h, v)
		}
	}
}

func (server CacheServer) HashURL(url string) string {
	hasher := md5.New()
	hasher.Write([]byte(server.CacheKeyPrefix + "/" + url))
	return hex.EncodeToString(hasher.Sum(nil))
}

func (server CacheServer) CheckCache(url string) []byte {
	if server.Repo == nil {
		return nil
	}
	return server.Repo.Get(url)
}

func (server CacheServer) ReorderQueryString(url *url.URL) string {
	return url.Path + "?" + url.Query().Encode()
}
