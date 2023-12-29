FROM golang:bookworm as builder
RUN apt install sqlite3
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . . 
RUN go build -ldflags '-extldflags "-static"' main.go
RUN mkdir /root/stuff

VOLUME /app/stuff

ARG DISCORD_API_TOKEN
ARG CONFIG_PATH
CMD ["/app/main"]
