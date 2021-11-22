package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zeriontech/sidecache/pkg/cache"
	"go.uber.org/zap"
)

const applicationDefaultPort = ":9191"

type CacheServer struct {
	Repo           cache.Repository
	Proxy          *httputil.ReverseProxy
	Prometheus     *Prometheus
	Logger         *zap.Logger
	CacheKeyPrefix string
}

type CacheData struct {
	Body    []byte
	Headers map[string]string
}

func NewServer(repo cache.Repository, proxy *httputil.ReverseProxy, prom *Prometheus, logger *zap.Logger) *CacheServer {
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
		if r.StatusCode >= 500 {
			return nil
		}

		maxAgeInSecond, err := time.ParseDuration(os.Getenv("CACHE_TTL"))

		if err != nil {
			server.Logger.Error("invalid cache TTL", zap.Error(err))
			return nil
		}

		r.Header.Del("Content-Length") // https://github.com/golang/go/issues/14975
		var b []byte
		if r.Header.Get("content-encoding") == "gzip" {
			reader, _ := gzip.NewReader(r.Body)
			b, err = ioutil.ReadAll(reader)
		} else {
			b, err = ioutil.ReadAll(r.Body)
		}

		if err != nil {
			server.Logger.Error("Error while reading response body", zap.Error(err))
			return err
		}

		buf := server.gzipWriter(b)
		go func(reqUrl *url.URL, data []byte, ttl time.Duration) {
			hashedURL := server.HashURL(server.ReorderQueryString(reqUrl))
			cacheData := CacheData{Body: data}
			cacheDataBytes, _ := json.Marshal(cacheData)
			server.Repo.SetKey(hashedURL, cacheDataBytes, ttl)
		}(r.Request.URL, buf.Bytes(), maxAgeInSecond)

		err = r.Body.Close()
		if err != nil {
			server.Logger.Error("Error while closing response body", zap.Error(err))
			return err
		}

		var body io.ReadCloser
		if r.Header.Get("content-encoding") == "gzip" {
			body = ioutil.NopCloser(buf)
		} else {
			body = ioutil.NopCloser(bytes.NewReader(b))
		}

		r.Body = body

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
	if _, err := gzipWriter.Write(b); err != nil {
		server.Logger.Error("Gzip writer encountered an error", zap.Error(err))
	}
	if err := gzipWriter.Close(); err != nil {
		server.Logger.Error("Gzip writer is not closed", zap.Error(err))
		return nil
	}
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
	cachedDataBytes := server.CheckCache(hashedURL)

	server.Logger.Info("serve request", zap.String("url", r.URL.String()), zap.Bool("cached", cachedDataBytes != nil))

	if cachedDataBytes != nil {
		w.Header().Add("X-Cache-Response-For", r.URL.String())
		w.Header().Add("Content-Type", "application/json;charset=UTF-8") //todo get from cache?

		var cachedData CacheData
		err := json.Unmarshal(cachedDataBytes, &cachedData)
		if err != nil {
			//backward compatibility
			//if we can not marshall cached data to new structure
			//we write previously cached byte data
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				reader, _ := gzip.NewReader(bytes.NewReader(cachedDataBytes))
				if _, err := io.Copy(w, reader); err != nil {
					server.Logger.Error("IO error", zap.Error(err))
					return
				}
			} else {
				w.Header().Add("Content-Encoding", "gzip")
				if _, err := io.Copy(w, bytes.NewReader(cachedDataBytes)); err != nil {
					server.Logger.Error("IO error", zap.Error(err))
					return
				}
			}
		} else {
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				reader, _ := gzip.NewReader(bytes.NewReader(cachedData.Body))
				delete(cachedData.Headers, "Content-Encoding")
				writeHeaders(w, cachedData.Headers)
				if _, err := io.Copy(w, reader); err != nil {
					server.Logger.Error("IO error", zap.Error(err))
					return
				}
			} else {
				writeHeaders(w, cachedData.Headers)
				if _, ok := cachedData.Headers["Content-Encoding"]; !ok {
					w.Header().Add("Content-Encoding", "gzip")
				}
				if _, err := io.Copy(w, bytes.NewReader(cachedData.Body)); err != nil {
					server.Logger.Error("IO error", zap.Error(err))
					return
				}
			}
		}

		server.Prometheus.CacheHitCounter.Inc()
	} else {
		server.Proxy.ServeHTTP(w, r)
	}
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
