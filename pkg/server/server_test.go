package server_test

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"github.com/golang/mock/gomock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/zeriontech/sidecache/pkg/cache"
	"github.com/zeriontech/sidecache/pkg/server"
	"go.uber.org/zap"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"testing"
	"time"
)

var apiUrl, _ = url.Parse("http://localhost:8080/")
var proxy = httputil.NewSingleHostReverseProxy(apiUrl)
var logger, _ = zap.NewProduction()
var client = server.NewPrometheusClient()
var cacheServer *server.CacheServer
var repos cache.Repository

func TestMain(m *testing.M) {
	client.CacheHitCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "sidecache",
			Name:      "cache_hit_counter",
			Help:      "This is my counter",
		})

	listener, _ := net.Listen("tcp", "127.0.0.1:8080")
	fakeApiServer := httptest.NewUnstartedServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-TTL", "300")
			w.Header().Set("tysidecarcachable", "ttl=300")
			w.Header().Set("sidecache-headers-enabled", "true")

			user := map[string]string{
				"Id":    "1",
				"Name":  "Emre Savcı",
				"Email": "emre.savci@trendyol.com",
				"Phone": "000099999",
			}
			json.NewEncoder(w).Encode(user)
		}))

	cacheServer = server.NewServer(repos, proxy, client, logger)
	stopChan := make(chan int)
	go cacheServer.Start(stopChan)

	fakeApiServer.Listener.Close()
	fakeApiServer.Listener = listener
	fakeApiServer.Start()

	code := m.Run()

	fakeApiServer.Close()
	stopChan <- 1
	os.Exit(code)
}

func TestReorderQueryString(t *testing.T) {
	var firstURL *url.URL
	var cacheServer *server.CacheServer
	var reorderQueryString string
	firstURL, _ = url.Parse("http://localhost:8080/api?year=2020&name=emre")
	cacheServer = server.NewServer(nil, nil, nil, nil)

	reorderQueryString = cacheServer.ReorderQueryString(firstURL)
	if reorderQueryString != "/api?name=emre&year=2020" {
		t.Errorf("Query strings are not equal")
	}
}

func TestHashUrl(t *testing.T) {
	var expectedHash string
	var actualHash string
	var cacheServer *server.CacheServer

	cacheServer = server.NewServer(nil, nil, nil, nil)
	cacheServer.CacheKeyPrefix = "test-prefix"

	testUrl := "testurl"

	hasher := md5.New()
	hasher.Write([]byte("test-prefix" + "/" + testUrl))
	expectedHash = hex.EncodeToString(hasher.Sum(nil))

	actualHash = cacheServer.HashURL(testUrl)

	if expectedHash != actualHash {
		t.Errorf("Hashes are not equal")
	}
}

func TestGetTTL(t *testing.T) {
	var value int
	var cacheServer *server.CacheServer
	cacheServer = server.NewServer(nil, nil, nil, nil)
	value = cacheServer.GetHeaderTTL("max-age=100")

	if value != 100 {
		t.Errorf("TTL values are not equal")
	}
}

func TestReturnProxyResponseWhenRepoReturnsNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := cache.NewMockCacheRepository(ctrl)
	repo.
		EXPECT().
		Get(gomock.Any()).
		Return(nil)

	repo.
		EXPECT().
		SetKey(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1)

	cacheServer = server.NewServer(repo, proxy, client, logger)
	stopChan := make(chan int)
	go cacheServer.Start(stopChan)
	time.Sleep(5 * time.Second)
	resp, _ := http.Get("http://localhost:9191/api?name=emre&year=2020")
	respBody, _ := ioutil.ReadAll(resp.Body)

	actual := make(map[string]string)

	json.Unmarshal(respBody, &actual)

	if actual["Email"] != "emre.savci@trendyol.com" {
		t.Errorf("Email is not equal to expected")
	}
	stopChan <- 1
}

func TestReturnCacheResponseWhenRepoReturnsData(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := cache.NewMockCacheRepository(ctrl)
	cacheServer.Repo = repo

	str := "{'name':'emre'}"
	buf := bytes.NewBuffer([]byte{})
	gzipWriter := gzip.NewWriter(buf)
	gzipWriter.Write([]byte(str))
	gzipWriter.Close()

	repo.
		EXPECT().
		Get(gomock.Any()).
		Return(buf.Bytes())

	resp, _ := http.Get("http://localhost:9191/api?name=emre&year=2020")
	respBody, _ := ioutil.ReadAll(resp.Body)

	if string(respBody) != str {
		t.Errorf("Bodies are not equal")
	}
}

func TestReturnProxyResponseWhenNoCacheHeaderExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := cache.NewMockCacheRepository(ctrl)
	cacheServer.Repo = repo

	repo.
		EXPECT().
		Get(gomock.Any()).
		Return(nil)

	repo.
		EXPECT().
		SetKey(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1)

	httpClient := &http.Client{}
	req, _ := http.NewRequest("GET", "http://localhost:9191/api?name=emre&year=2020", nil)
	req.Header.Add("X-No-Cache", "true")
	resp, _ := httpClient.Do(req)

	respBody, _ := ioutil.ReadAll(resp.Body)

	actual := make(map[string]string)

	json.Unmarshal(respBody, &actual)

	if actual["Email"] != "emre.savci@trendyol.com" {
		t.Errorf("Email is not equal to expected")
	}
}

func TestReturnCacheHeadersWhenCacheHeaderEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := cache.NewMockCacheRepository(ctrl)
	cacheServer.Repo = repo

	user := map[string]string{
		"Id":    "1",
		"Name":  "Emre Savcı",
		"Email": "emre.savci@trendyol.com",
		"Phone": "000099999",
	}

	userByte, _ := json.Marshal(user)
	headers := map[string]string{
		"Content-Type":              "application/json",
		"Cache-TTL":                 "300",
		"sidecache-headers-enabled": "true",
	}

	cacheData := server.CacheData{
		Body:    userByte,
		Headers: headers,
	}

	cacheDataBytes, _ := json.Marshal(cacheData)

	repo.
		EXPECT().
		Get(gomock.Any()).
		Return(nil)

	repo.
		EXPECT().
		SetKey(gomock.Any(), gomock.Eq(cacheDataBytes), gomock.Any()).
		Times(1)

	httpClient := &http.Client{}
	req, _ := http.NewRequest("GET", "http://localhost:9191/api?name=emre&year=2020", nil)
	resp, _ := httpClient.Do(req)

	respBody, _ := ioutil.ReadAll(resp.Body)

	actual := make(map[string]string)

	json.Unmarshal(respBody, &actual)
	if actual["Email"] != "emre.savci@trendyol.com" {
		t.Errorf("Email is not equal to expected")
	}
}
