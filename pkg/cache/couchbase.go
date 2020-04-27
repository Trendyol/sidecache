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
	// TODO timeouts
	cluster, err := gocb.Connect("couchbase://" + os.Getenv("COUCHBASE_HOST"))
	if err != nil {
		fmt.Printf("Couchbase connection error: %s\n", err)
		return nil
	}

	err = cluster.Authenticate(gocb.PasswordAuthenticator{
		Username: os.Getenv("COUCHBASE_USERNAME"),
		Password: os.Getenv("COUCHBASE_PASSWORD"),
	})

	cacheBucket, err := cluster.OpenBucket(os.Getenv("BUCKET_NAME"), "")
	if err != nil {
		fmt.Println(err)
		return nil
	}

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
