.DEFAULT_GOAL := all

GOOS?=linux
GOARCH?=amd64
CGO_ENABLED?=1

.PHONY: all
all: build

.PHONY: govendor
govendor:
	go get github.com/kardianos/govendor
	govendor sync
	# TODO: Remove the dependency "github.com/denisenkom/go-mssqldb"
	sed -i -e 's/+build !windows//g' vendor/github.com/denisenkom/go-mssqldb/ntlm.go
	sed -i -e 's/ntlmAuth/NTLMAuth/g' vendor/github.com/denisenkom/go-mssqldb/ntlm.go

.PHONY: build
build: govendor
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build

.PHONY: testall
testall: govendor
	go test