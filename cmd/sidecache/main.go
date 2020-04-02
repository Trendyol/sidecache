package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Trendyol/sidecache/pkg/cache"
)

type CacheRequest struct {
	Key  string
	Data interface{}
	TTL  int
}

func main() {
	srv := &http.Server{Addr: ":8080"}

	repo := cache.NewRedisRepository()

	http.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var request CacheRequest

			requestBody, err := ioutil.ReadAll(r.Body)

			if err != nil {
				fmt.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

			json.Unmarshal(requestBody, &request)
			repo.SetKeyTTL(request.Key, request.Data, request.TTL)
		}
	})

	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")

			key := r.URL.Query().Get("key")

			var data interface{}
			repo.Get(key, &data)

			json.NewEncoder(w).Encode(data)
		}
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			fmt.Printf("Httpserver: ListenAndServe() error: %s \n", err)
		}
	}()

	<-make(chan int, 0)
}
