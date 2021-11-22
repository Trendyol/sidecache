-include .env
export

.PHONY: run
run:
	go run -v cmd/sidecache/main.go
