package server_test

import (
	"crypto/md5"
	"encoding/hex"
	"net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

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

		testUrl := "testurl"

		hasher := md5.New()
		hasher.Write([]byte(testUrl))
		expectedHash = hex.EncodeToString(hasher.Sum(nil))

		actualHash = cacheServer.HashURL(testUrl)
	})

	It("should hash url", func() {
		Expect(expectedHash).To(Equal(actualHash))
	})
})
