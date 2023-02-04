package tests

import (
	"github.com/Trendyol/sidecache/pkg/server"
	"os"
	"testing"
)

func BenchmarkServerHash(b *testing.B) {
	os.Setenv("CACHE_KEY_PREFIX", "test")
	var cacheServer = new(server.CacheServer)

	b.ResetTimer()
	
	for n := 0; n < b.N; n++ {
		cacheServer.HashURL("adsfadsdfasdfas")
	}
}
