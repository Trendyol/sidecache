package server

import (
	"bytes"
	"compress/gzip"
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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

const CacheHeaderKey = "tysidecarcachable"
const applicationDefaultPort = ":9191"

type CacheServer struct {
	Repo           cache.CacheRepository
	Proxy          *httputil.ReverseProxy
	Prometheus     *Prometheus
	Logger         *zap.Logger
	CacheKeyPrefix string
}

func NewServer(repo cache.CacheRepository, proxy *httputil.ReverseProxy, prom *Prometheus, logger *zap.Logger) *CacheServer {
	return &CacheServer{
		Repo:           repo,
		Proxy:          proxy,
		Prometheus:     prom,
		Logger:         logger,
		CacheKeyPrefix: os.Getenv("CACHE_KEY_PREFIX"),
	}
}

func (server CacheServer) Start(stopChan chan int) {
	server.Proxy.ModifyResponse = func(r *http.Response) error {
		cacheHeaderValue := r.Header.Get(CacheHeaderKey)
		if cacheHeaderValue != "" {
			maxAgeInSecond := server.GetHeaderTTL(cacheHeaderValue)
			r.Header.Del("Content-Length") // https://github.com/golang/go/issues/14975
			var b []byte
			var err error
			if r.Header.Get("content-encoding") == "gzip" {
				reader, _ := gzip.NewReader(r.Body)
				b, err = ioutil.ReadAll(reader)
			} else {
				b, err = ioutil.ReadAll(r.Body)
			}

			if err != nil {
				server.Logger.Error("Error while reading repsonse body", zap.Error(err))
				return err
			}

			buf := server.gzipWriter(b)
			go func(reqUrl *url.URL, data []byte, ttl int) {
				hashedURL := server.HashURL(server.ReorderQueryString(reqUrl))
				server.Repo.SetKey(hashedURL, data, ttl)
			}(r.Request.URL, buf.Bytes(), maxAgeInSecond)

			err = r.Body.Close()
			if err != nil {
				server.Logger.Error("Error while closing repsonse body", zap.Error(err))
				return err
			}

			var body io.ReadCloser
			if r.Header.Get("content-encoding") == "gzip" {
				body = ioutil.NopCloser(buf)
			} else {
				body = ioutil.NopCloser(bytes.NewReader(b))
			}

			r.Body = body
		}

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

func (server CacheServer) gzipWriter(b []byte) *bytes.Buffer {
	buf := bytes.NewBuffer([]byte{})
	gzipWriter := gzip.NewWriter(buf)
	_, err := gzipWriter.Write(b)
	if err != nil {
		server.Logger.Error("Gzip Writer Encountered With an Error", zap.Error(err))
	}
	gzipWriter.Close()
	return buf
}

func (server CacheServer) CacheHandler(w http.ResponseWriter, r *http.Request) {
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

	hashedURL := server.HashURL(server.ReorderQueryString(r.URL))
	cachedData := server.CheckCache(hashedURL)

	if cachedData != nil {
		w.Header().Add("X-Cache-Response-For", r.URL.String())
		w.Header().Add("Content-Type", "application/json;charset=UTF-8")

		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			reader, _ := gzip.NewReader(bytes.NewReader(cachedData))
			io.Copy(w, reader)
		} else {
			w.Header().Add("Content-Encoding", "gzip")
			io.Copy(w, bytes.NewReader(cachedData))
		}

		server.Prometheus.CacheHitCounter.Inc()
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
	if server.Repo == nil {
		return nil
	}
	return server.Repo.Get(url)
}

func (server CacheServer) ReorderQueryString(url *url.URL) string {
	return url.Path + "?" + url.Query().Encode()
}
