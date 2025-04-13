FROM golang:latest AS builder
COPY ./ /go/src/build
WORKDIR /go/src/build
RUN go mod vendor
ENV CGO_ENABLED=0
RUN GOOS=linux go build -mod vendor -a -o smtp2http .

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /go/src/build/smtp2http /usr/bin/smtp2http

ENV WEBHOOK_URL=none
ENTRYPOINT smtp2http --webhook=${WEBHOOK_URL}
