package cache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis"
)

type RedisRepository struct {
	client *redis.Client
}

func NewRedisRepository() *RedisRepository {
	client := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("redisAddr"),
		Password: os.Getenv("redisPassword"),
		DB:       0,
	})

	return &RedisRepository{client: client}
}

func (repository *RedisRepository) SetKey(key string, value interface{}) {
	byteData, err := json.Marshal(value)

	if err != nil {
		fmt.Println(err)
		return
	}

	status := repository.client.Set(key, string(byteData), 0)
	_, err = status.Result()
	if err != nil {
		fmt.Println(err)
	}
}

func (repository *RedisRepository) SetKeyTTL(key string, value interface{}, ttl int) {
	byteData, err := json.Marshal(value)

	if err != nil {
		fmt.Println(err)
		return
	}

	duration, _ := time.ParseDuration(strconv.FormatInt(int64(ttl), 10))
	status := repository.client.Set(key, string(byteData), duration)
	_, err = status.Result()
	if err != nil {
		fmt.Println(err)
	}
}

func (repository *RedisRepository) Get(key string, data interface{}) {
	status := repository.client.Get(key)
	stringResult, err := status.Result()
	if err != nil {
		fmt.Println(err)
	}

	json.Unmarshal(bytes.NewBufferString(stringResult).Bytes(), data)
}
