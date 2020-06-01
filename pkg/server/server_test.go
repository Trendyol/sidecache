package server_test

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/Trendyol/sidecache/pkg/cache"
	"github.com/Trendyol/sidecache/pkg/server"
)

var _ = Describe("Server", func() {

	var firstURL *url.URL
	var cacheServer *server.CacheServer
	var reorderQueryString string

	BeforeEach(func() {
		firstURL, _ = url.Parse("http://localhost:8080/api?year=2020&name=emre")
		cacheServer = server.NewServer(nil, nil, nil, nil)

		reorderQueryString = cacheServer.ReorderQueryString(firstURL)
	})

	It("should reorder querystring of url", func() {
		Expect(reorderQueryString).To(Equal("/api?name=emre&year=2020"))
	})
})

var _ = Describe("Server", func() {

	var expectedHash string
	var actualHash string
	var cacheServer *server.CacheServer

	BeforeEach(func() {
		cacheServer = server.NewServer(nil, nil, nil, nil)
		cacheServer.CacheKeyPrefix = "test-prefix"

		testUrl := "testurl"

		hasher := md5.New()
		hasher.Write([]byte("test-prefix" + "/" + testUrl))
		expectedHash = hex.EncodeToString(hasher.Sum(nil))

		actualHash = cacheServer.HashURL(testUrl)
	})

	It("should hash url", func() {
		Expect(expectedHash).To(Equal(actualHash))
	})
})

var _ = Describe("Server", func() {

	var cacheServer *server.CacheServer
	stopChan := make(chan (int))
	var repo *cache.MockCacheRepository

	var once sync.Once
	var afterAllCount int

	BeforeEach(func() {
		once.Do(func() {

			ctrl := gomock.NewController(GinkgoT())
			repo = cache.NewMockCacheRepository(ctrl)
			apiUrl, _ := url.Parse("http://localhost:8080/")
			proxy := httputil.NewSingleHostReverseProxy(apiUrl)
			logger, _ := zap.NewProduction()
			counter := prometheus.NewCounter(
				prometheus.CounterOpts{
					Namespace: "sidecache",
					Name:      "cache_hit_counter",
					Help:      "This is my counter",
				})

			cacheServer = server.NewServer(repo, proxy, counter, logger)
			go cacheServer.Start(stopChan)

			time.Sleep(5 * time.Second)
		})
	})

	AfterEach(func() {
		afterAllCount++
		fmt.Println(afterAllCount)
		if afterAllCount == 3 {
			stopChan <- 1
		}
	})

	It("should return cached response", func() {
		repo.
			EXPECT().
			Get(gomock.Any()).
			Return([]byte("{'name':'emre'}"))

		resp, _ := http.Get("http://localhost:9191/api?name=emre&year=2020")
		respBody, _ := ioutil.ReadAll(resp.Body)

		Expect(string(respBody)).To(Equal("{'name':'emre'}"))
	})

	It("should return proxy response", func() {
		repo.
			EXPECT().
			Get(gomock.Any()).
			Return(nil)

		repo.
			EXPECT().
			SetKey(gomock.Any(), gomock.Any(), gomock.Any()).
			Times(1)

		resp, _ := http.Get("http://localhost:9191/api?name=emre&year=2020")
		respBody, _ := ioutil.ReadAll(resp.Body)

		actual := make(map[string]string)

		json.Unmarshal(respBody, &actual)

		Expect(actual["Email"]).To(Equal("emre.savci@trendyol.com"))
	})

	It("should return proxy response when X-No-Cache header exists on request", func() {
		repo.
			EXPECT().
			Get(gomock.Any()).
			Return(nil)

		repo.
			EXPECT().
			SetKey(gomock.Any(), gomock.Any(), gomock.Any()).
			Times(1)

		client := &http.Client{}
		req, _ := http.NewRequest("GET", "http://localhost:9191/api?name=emre&year=2020", nil)
		req.Header.Add("X-No-Cache", "true")
		resp, _ := client.Do(req)

		respBody, _ := ioutil.ReadAll(resp.Body)

		actual := make(map[string]string)

		json.Unmarshal(respBody, &actual)

		Expect(actual["Email"]).To(Equal("emre.savci@trendyol.com"))
	})
})

var _ = Describe("Server", func() {

	var value int
	var cacheServer *server.CacheServer

	BeforeEach(func() {
		cacheServer = server.NewServer(nil, nil, nil, nil)

		value = cacheServer.GetHeaderTTL("max-age=100")
	})

	It("should get cache ttl in second", func() {
		Expect(value).To(Equal(100))
	})
})
