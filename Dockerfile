FROM golang:1.19.3-alpine3.16

ENV CGO_ENABLED=0

WORKDIR /usr/src/form3api
COPY . /usr/src/form3api
