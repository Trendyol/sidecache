package cache

import (
	"fmt"
	"os"

	"gopkg.in/couchbase/gocb.v1"
)

type CouchbaseRepository struct {
	bucket *gocb.Bucket
}

func NewCouchbaseRepository() *CouchbaseRepository {
	cluster, err := gocb.Connect("couchbase://" + os.Getenv("couchbaseHost"))
	if err != nil {
		fmt.Println(err)
		return nil
	}

	err = cluster.Authenticate(gocb.PasswordAuthenticator{
		Username: os.Getenv("couchbaseUsername"),
		Password: os.Getenv("couchbasePassword"),
	})

	cacheBucket, err := cluster.OpenBucket(os.Getenv("bucketName"), "")
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return &CouchbaseRepository{bucket: cacheBucket}
}

func (repository *CouchbaseRepository) SetKey(key string, value interface{}) {
	_, err := repository.bucket.Insert(key, value, 0)
	if err != nil {
		fmt.Println(err)
	}
}

func (repository *CouchbaseRepository) SetKeyTTL(key string, value interface{}, ttl int) {
	_, err := repository.bucket.Insert(key, value, uint32(ttl))
	if err != nil {
		fmt.Println(err)
	}
}

func (repository *CouchbaseRepository) Get(key string, data interface{}) {
	_, err := repository.bucket.Get(key, &data)
	if err != nil {
		fmt.Println(err)
	}
}
