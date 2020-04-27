package cache

type CacheRepository interface {
	SetKey(key string, value []byte, ttl int)
	Get(key string) []byte
}
