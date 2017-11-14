REPO ?= github.com/codekoala/aws-roll
TAG ?= $(shell git rev-parse HEAD)-dev
BUILD_DATE := $(shell date +%FT%T%z)

all: clean build compress checksums

build: bin
	go build -ldflags '-s -X $(REPO)/version.Commit=$(TAG) -X $(REPO)/version.BuildDate=$(BUILD_DATE)' -o ./bin/roll ./cmd/roll

checksums: bin
	cd ./bin/; sha256sum roll* > SHA256SUMS

compress:
	upx ./bin/*

test:
	go test -race -cover `go list ./... | grep -v vendor`

bin:
	@mkdir -p ./bin

clean:
	rm -rf ./bin

.PHONY: bin
