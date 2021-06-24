FROM registry.trendyol.com/platform/base/image/golang:1.13.4-alpine3.10 AS builder

ENV GOPATH /go
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ARG VERSION

RUN mkdir /app
WORKDIR /app

COPY . .
RUN go mod download
RUN go build -ldflags="-X 'main.version=$VERSION'" -v cmd/sidecache/main.go

FROM registry.trendyol.com/platform/base/image/alpine:3.10.1 AS alpine

ENV LANG C.UTF-8
ENV MAIN_CONTAINER_PORT ""
ENV COUCHBASE_HOST ""
ENV COUCHBASE_USERNAME ""
ENV COUCHBASE_PASSWORD ""
ENV BUCKET_NAME ""

RUN apk --no-cache add tzdata ca-certificates
COPY --from=builder /app/main   /app/main

WORKDIR /app

RUN chmod +x main

EXPOSE 9191

ENTRYPOINT ["./main","app"]
