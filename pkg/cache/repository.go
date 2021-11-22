package cache

type Repository interface {
	SetKey(key string, value []byte, ttl int)
	Get(key string) []byte
}
