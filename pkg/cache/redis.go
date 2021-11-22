package cache

import (
	"go.uber.org/zap"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis"
)

type RedisRepository struct {
	client *redis.Client
	logger *zap.Logger
}

func NewRedisRepository(logger *zap.Logger) (*RedisRepository, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("redisAddr"),
		Password: os.Getenv("redisPassword"),
		DB:       0,
	})
	if err := client.Ping().Err(); err != nil {
		return nil, err
	}

	return &RedisRepository{client: client, logger: logger}, nil
}

func (repository *RedisRepository) SetKey(key string, value []byte, ttl int) {
	duration, _ := time.ParseDuration(strconv.FormatInt(int64(ttl), 10))
	status := repository.client.Set(key, string(value), duration)
	_, err := status.Result()
	if err != nil {
		repository.logger.Error(err.Error())
	}
}

func (repository *RedisRepository) Get(key string) []byte {
	status := repository.client.Get(key)
	stringResult, err := status.Result()
	if err != nil {
		repository.logger.Error(err.Error())
	}

	return []byte(stringResult)
}
