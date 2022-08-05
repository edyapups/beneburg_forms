# syntax=docker/dockerfile:1

## Build
FROM golang:1.18-buster AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
COPY vendor ./

COPY cmd/main.go ./

RUN go build -o /main

## Deploy
FROM gcr.io/distroless/base-debian10

WORKDIR /

COPY --from=build /main /main

EXPOSE 3333

ENTRYPOINT ["/main"]