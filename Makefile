GO:=go
GOPATH:=$(shell $(GO) env GOPATH)
PATH:=$(GOPATH)/bin:${PATH}
export GO111MODULE:=on
export CGO_ENABLED:=0

all: build

.PHONY: build
build:
	$(GO) build -v -o bin/k8s-zfs cmd/main.go

.PHONY: linux
linux:
	env GOOS=linux GOARCH=amd64 $(GO) build -v -o bin/k8s-zfs-linux cmd/main.go
