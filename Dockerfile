FROM golang:1.17.3@sha256:b5bfe0255e6fac7cec1abd091b5cc3a5c40e2ae4d09bafbe5e94cb705647f0fc as builder

ENV GO111MODULE=on \
    CGO_ENABLED=0  \
    GOARCH="amd64" \
    GOOS=linux

WORKDIR /app

# Copy and download dependency using go mod
COPY go.mod .
COPY go.sum .
RUN go mod download
RUN go mod verify

# Copy the code into the container
COPY . .

# Build the app
RUN go build -v cmd/sidecache/main.go

FROM gcr.io/distroless/base

ARG release_version
ENV RELEASE_VERSION=$release_version
ENV LANG C.UTF-8
ENV MAIN_CONTAINER_PORT "80"
ENV REDIS_ADDRESS "127.0.0.1:6379"
ENV REDIS_PASSWORD ""

COPY --from=builder /app/main /app/main

EXPOSE 9191

ENTRYPOINT ["/app/main", "app"]
