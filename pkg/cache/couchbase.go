package cache

import (
	"os"
	"time"

	"go.uber.org/zap"

	"gopkg.in/couchbase/gocb.v1"
)

type CouchbaseRepository struct {
	bucket *gocb.Bucket
	logger *zap.Logger
}

func NewCouchbaseRepository(logger *zap.Logger) *CouchbaseRepository {
	couchbaseHost := os.Getenv("COUCHBASE_HOST")
	cluster, err := gocb.Connect("couchbase://" + couchbaseHost)
	if err != nil {
		logger.Error("Couchbase connection error:", zap.Error(err))
		return nil
	}

	err = cluster.Authenticate(gocb.PasswordAuthenticator{
		Username: os.Getenv("COUCHBASE_USERNAME"),
		Password: os.Getenv("COUCHBASE_PASSWORD"),
	})
	cacheBucket, err := cluster.OpenBucket(os.Getenv("BUCKET_NAME"), "")
	if err != nil {
		logger.Error("Couchbase username or password  error:", zap.Error(err))
		return nil
	}
	cacheBucket.SetOperationTimeout(100 * time.Millisecond)

	return &CouchbaseRepository{bucket: cacheBucket, logger: logger}
}

func (repository *CouchbaseRepository) SetKey(key string, value []byte, ttl int) {
	_, err := repository.bucket.Upsert(key, value, uint32(ttl))
	if err != nil {
		repository.logger.Error("Error occurred when Upsert", zap.String("key", key))
	}
}

func (repository *CouchbaseRepository) Get(key string) []byte {
	var data []byte
	_, err := repository.bucket.Get(key, &data)

	if err != nil && err.Error() != "key not found" {
		repository.logger.Warn("Error occurred when Get", zap.String("key", key), zap.Error(err))
	}

	return data
}
