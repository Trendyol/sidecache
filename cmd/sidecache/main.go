package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Trendyol/sidecache/pkg/cache"
)

type CacheRequest struct {
	CacheKey string `json: "cacheKey"`
	Data     []byte `json: "data"`
	TTL      int    `json: "ttl"`
}

func main() {
	srv := &http.Server{Addr: ":9090"}

	repo := cache.NewRedisRepository()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var request CacheRequest
			requestBody, err := ioutil.ReadAll(r.Body)

			if err != nil {
				fmt.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			json.Unmarshal(requestBody, &request)
			repo.SetKeyTTL(request.CacheKey, request.Data, request.TTL)
		} else if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")

			key := r.URL.Query().Get("cacheKey")
			data := repo.Get(key)

			w.Write(data)
		}
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			fmt.Printf("Httpserver: ListenAndServe() error: %s \n", err)
		}
	}()

	<-make(chan int, 0)
}
