package cache

import (
	"time"
)

type Repository interface {
	SetKey(key string, value []byte, ttl time.Duration)
	Get(key string) []byte
}
