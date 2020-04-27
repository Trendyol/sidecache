package cache

type CacheRepository interface {
	SetKey(key string, value interface{}, ttl int)
	Get(key string) []byte
}
