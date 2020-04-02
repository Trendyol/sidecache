FROM golang:1.13.4-alpine AS builder

ENV GOPATH /go
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN mkdir /app
WORKDIR /app

COPY . .
RUN go mod download
RUN go build -v cmd/main.go

FROM alpine:latest AS alpine

ENV LANG C.UTF-8
ENV GOPATH /go

RUN apk --no-cache add tzdata ca-certificates
COPY --from=builder /app/main   /app/main

WORKDIR /app

RUN chmod +x main

EXPOSE 8080

ENTRYPOINT ["./main","app"]




