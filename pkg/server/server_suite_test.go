package server_test

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Suite")
}

var fakeApiServer *httptest.Server

var _ = BeforeSuite(func() {

	listener, _ := net.Listen("tcp", "127.0.0.1:8080")

	fakeApiServer = httptest.NewUnstartedServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-TTL", "300")

			user := map[string]string{
				"Id":    "1",
				"Name":  "Emre Savcı",
				"Email": "emre.savci@trendyol.com",
				"Phone": "000099999",
			}
			json.NewEncoder(w).Encode(user)
		}))

	fakeApiServer.Listener.Close()
	fakeApiServer.Listener = listener
	fakeApiServer.Start()
})

var _ = AfterSuite(func() {
	fakeApiServer.Close()
})
