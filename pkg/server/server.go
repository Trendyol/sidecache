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
	"time"

	"github.com/Trendyol/sidecache/pkg/cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

type CacheServer struct {
	Repo    cache.CacheRepository
	Proxy   *httputil.ReverseProxy
	Counter prometheus.Counter
	Logger  *zap.Logger
}

func NewServer(repo cache.CacheRepository, proxy *httputil.ReverseProxy, counter prometheus.Counter, logger *zap.Logger) *CacheServer {
	return &CacheServer{
		Repo:    repo,
		Proxy:   proxy,
		Counter: counter,
		Logger:  logger,
	}
}

func (server CacheServer) Start(stopChan chan (int)) {
	server.Proxy.ModifyResponse = func(r *http.Response) error {
		defer server.elapsed("ModifyResponse")()
		//if r.Header.Get("Cache-TTL") == "300" {

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return err
		}

		go func(reqUrl *url.URL, data []byte) {
			hashedURL := server.HashURL(server.ReorderQueryString(reqUrl))
			server.Repo.SetKey(hashedURL, data, 0)
		}(r.Request.URL, b)

		err = r.Body.Close()
		if err != nil {
			return err
		}

		body := ioutil.NopCloser(bytes.NewReader(b))
		r.Body = body
		//}

		return nil
	}

	http.HandleFunc("/", server.CacheHandler)
	http.Handle("/metrics", promhttp.Handler())

	httpServer := &http.Server{Addr: ":9191"}

	go func() {
		//server.Logger.Fatal("Error while starting server: ", zap.Error(http.ListenAndServe(":9191", nil)))
		server.Logger.Warn("Server closed: ", zap.Error(httpServer.ListenAndServe()))
	}()

	<-stopChan

	httpServer.Shutdown(context.Background())
}

func (server CacheServer) elapsed(methodName string) func() {
	start := time.Now()
	return func() {
		server.Logger.Info("",
			zap.String("MethodName", methodName),
			zap.Int64("ElapsedTime in MS", time.Since(start).Milliseconds()),
		)
	}
}

func (server CacheServer) CacheHandler(w http.ResponseWriter, r *http.Request) {
	defer server.elapsed("CacheHandler")() // <-- The trailing () is the deferred call

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
		server.Logger.Info("Cache found")
		w.Header().Add("X-Cache-Response-For", r.URL.String())
		io.Copy(w, bytes.NewBuffer(cachedData))
		server.Counter.Inc()
	} else {
		server.Proxy.ServeHTTP(w, r)
	}
}

func (server CacheServer) HashURL(url string) string {
	// TODO app name prefix
	hasher := md5.New()
	hasher.Write([]byte(url))
	return hex.EncodeToString(hasher.Sum(nil))
}

func (server CacheServer) CheckCache(url string) []byte {
	defer server.elapsed("checkCache")()
	return server.Repo.Get(url)
}

func (server CacheServer) ReorderQueryString(url *url.URL) string {
	return url.Path + "?" + url.Query().Encode()
}
