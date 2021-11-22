package tests

import (
	"github.com/zeriontech/sidecache/pkg/server"
	"os"
	"testing"
)

func BenchmarkServerHash(b *testing.B) {
	os.Setenv("CACHE_KEY_PREFIX", "test")
	var cacheServer *server.CacheServer = new(server.CacheServer)
	for n := 0; n < b.N; n++ {
		cacheServer.HashURL("adsfadsdfasdfas")
	}
}
