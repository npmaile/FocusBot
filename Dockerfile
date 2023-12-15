FROM golang:bookworm as builder

WORKDIR /root

COPY . . 

RUN go build main.go
