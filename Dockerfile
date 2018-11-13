FROM golang:alpine as builder
RUN set -x && apk --no-cache add make git && mkdir /src
COPY go.mod go.sum /src/
WORKDIR /src
RUN go mod download
COPY cmd /src/cmd
COPY pkg /src/pkg
COPY Makefile /src/
RUN make

FROM ubuntu:bionic
RUN apt-get update && apt-get install -y zfsutils-linux && rm -rf /var/lib/apt/lists
COPY --from=builder /src/bin/k8s-zfs /k8s-zfs
ENTRYPOINT ["/k8s-zfs"]
