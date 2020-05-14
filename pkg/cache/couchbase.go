package cache

import (
	"fmt"
	"go.uber.org/zap"
	"os"
	"time"

	"gopkg.in/couchbase/gocb.v1"
)

type CouchbaseRepository struct {
	bucket *gocb.Bucket
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
	cacheBucket.SetOperationTimeout(1 * time.Second)

	return &CouchbaseRepository{bucket: cacheBucket}
}

func (repository *CouchbaseRepository) SetKey(key string, value []byte, ttl int) {
	_, err := repository.bucket.Upsert(key, value, uint32(ttl))
	if err != nil {
		fmt.Println(err)
	}
}

func (repository *CouchbaseRepository) Get(key string) []byte {
	var data []byte
	_, err := repository.bucket.Get(key, &data)

	if err != nil {
		fmt.Println(err)
	}

	return data
}
