FROM golang:1.12-alpine3.9 as golang

ADD . /go/src/github.com/kubesphere/edge-watcher
WORKDIR /go/src/github.com/kubesphere/edge-watcher

RUN apk add --update --no-cache ca-certificates git

ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=linux

RUN mkdir -p /edge-watcher-agent
RUN go build -v -a -installsuffix cgo -ldflags '-w' -o /edge-watcher-agent/edge-watcher-agent main.go


FROM alpine:3.10
MAINTAINER KubeSphere <kubesphere@yunify.com>
LABEL Description="IPTables agent docker image" Vendor="KubeSphere Organization" Version="1.0"

RUN apk add --no-cache bash ca-certificates curl iptables && \
    mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

# Get IPTables wrapper from https://github.com/kubernetes-sigs/iptables-wrappers/
COPY iptables-wrapper-installer.sh /
RUN /iptables-wrapper-installer.sh

COPY --from=golang /edge-watcher-agent/edge-watcher-agent /edge-watcher-agent/edge-watcher-agent

ENTRYPOINT [ "/edge-watcher-agent/edge-watcher-agent" ]