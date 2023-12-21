FROM golang:bookworm as builder
WORKDIR /root
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . . 
RUN go build -ldflags '-extldflags "-static"' main.go
RUN mkdir /root/stuff

FROM scratch
WORKDIR /app
COPY --from=builder /root/main /app/focusbot
COPY --from=builder /root/stuff /app/stuff
COPY --from=builder /etc/ssl/certificates /etc/ssl/certificates
VOLUME /app/stuff

ARG DISCORD_API_TOKEN
ARG CONFIG_PATH
CMD ["/app/focusbot"]
