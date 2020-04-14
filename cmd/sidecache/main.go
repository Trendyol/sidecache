package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Trendyol/sidecache/pkg/cache"
)

type CacheRequest struct {
	CacheKey string `json: "cacheKey"`
	Data     []byte `json: "data"`
	TTL      int    `json: "ttl"`
}

func startHTTPServer() {
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

var httpResponseStart []byte
var repo *cache.CouchbaseRepository
var prxy *httputil.ReverseProxy

func main() {
	repo = cache.NewCouchbaseRepository()

	//mainContainerPort := os.Getenv("MAIN_CONTAINER_PORT")
	url, err := url.Parse("http://127.0.0.1:8080")
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(url == nil)
	fmt.Println(repo == nil)
	fmt.Println(url)
	fmt.Println(repo)

	prxy = httputil.NewSingleHostReverseProxy(url)

	prxy.ModifyResponse = func(r *http.Response) error {
		defer elapsed("ModifyResponse")()
		//if r.Header.Get("Cache-TTL") == "300" {
		fmt.Println("modify response")
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return err
		}

		go func(url string, data []byte) {
			hashedURL := hashURL(url)
			repo.SetKey(hashedURL, data)
		}(r.Request.URL.RequestURI(), b)

		err = r.Body.Close()
		if err != nil {
			return err
		}

		body := ioutil.NopCloser(bytes.NewReader(b))
		r.Body = body
		//}

		return nil
	}

	http.HandleFunc("/", CacheHandler)

	log.Fatal(http.ListenAndServe(":9191", nil))

	/*
		// TODO: only get requests

		httpResponseStart = []byte("HTTP/1.1 200 OK\nContent-Type: application/json;\nCustom-CacheProxy:True\nContent-Length:")

		listener, err := net.Listen("tcp", ":9191")
		if err != nil {
			fmt.Println(err)
			os.Exit(0)
		}
		defer listener.Close()

		for {
			conn, err := listener.Accept()

			if err != nil {
				fmt.Println(err)
			} else {
				go func() {

					// request al url extract et
					//
					var buffer bytes.Buffer
					teeReader := io.TeeReader(conn, &buffer)

					url := readURL(teeReader)
					hashedURL := hashURL(url)
					cachedData := checkCache(hashedURL)

					fmt.Println(hashedURL)

					if cachedData == nil {
						dataChannel := make(chan ([]byte))
						proxy(conn, &buffer, dataChannel)

						go func(ch chan ([]byte), url string) {
							data := <-ch
							repo.SetKey(url, data)
						}(dataChannel, hashedURL)
					} else {
						fmt.Println("data cachede bulundu")
						returnResponse(cachedData, conn)
					}
				}()
			}
		}
	*/

}

func elapsed(methodName string) func() {
	start := time.Now()
	return func() {
		fmt.Printf("%s took %v\n", methodName, time.Since(start))
	}
}

func CacheHandler(w http.ResponseWriter, r *http.Request) {
	defer elapsed("CacheHandler")() // <-- The trailing () is the deferred call

	defer func() {
		if rec := recover(); rec != nil {
			fmt.Printf("Recovered from panic: %v \n", rec)
			var err error
			switch x := rec.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("Unknown panic")
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}()

	hashedURL := hashURL(r.RequestURI)
	cachedData := checkCache(hashedURL)
	if cachedData != nil {
		fmt.Println("cache found")
		io.Copy(w, bytes.NewBuffer(cachedData))
	} else {
		prxy.ServeHTTP(w, r)
	}
}

func hashURL(url string) string {
	defer elapsed("hashURL")() // <-- The trailing () is the deferred call

	// TODO app name prefix
	// TODO order query param
	hasher := md5.New()
	hasher.Write([]byte(url))
	return hex.EncodeToString(hasher.Sum(nil))
}

func checkCache(url string) []byte {
	defer elapsed("checkCache")()
	var responseData []byte
	responseData = repo.Get(url, responseData)
	return responseData
}

func returnResponse(cachedData []byte, conn net.Conn) {
	conn.Write(httpResponseStart)
	responseLength := len(cachedData)
	conn.Write([]byte(strconv.Itoa(responseLength)))
	conn.Write([]byte("\r\n"))
	conn.Write([]byte("\r\n"))
	conn.Write(cachedData)
	conn.Close()
}

func readURL(reader io.Reader) string {
	sc := bufio.NewScanner(reader)
	sc.Scan()
	uri := sc.Text()
	return strings.Split(uri, " ")[1]
}

type ResponseWriter struct {
	hasCacheHeader bool
	ResponseBody   bytes.Buffer
}

func (rw *ResponseWriter) Write(response []byte) (int, error) {
	rw.ResponseBody.Write(response)
	return len(response), nil
}

func proxy(conn net.Conn, request io.Reader, dataChannel chan ([]byte)) {
	backend, err := net.Dial("tcp", "localhost:8080")

	if err != nil {
		fmt.Println(err)
	}

	go func() {
		io.Copy(backend, request)
	}()

	go func() {
		teeReader := io.TeeReader(backend, conn)

		r, err := http.ReadResponse(bufio.NewReader(teeReader), nil)
		if err != nil {
			fmt.Println(err)
		}

		if r.Header.Get("Cache-TTL") == "300" {
			btes, _ := ioutil.ReadAll(r.Body)

			if err == nil {
				dataChannel <- btes
			} else {
				fmt.Println(err)
			}
		}

		backend.Close()
		conn.Close()
	}()
}
