.DEFAULT_GOAL := build
BUILD_DATE = `date +%FT%T%z`
GO = go
BINARY_DIR=bin

GODIRS_NOVENDOR = $(shell go list ./... | grep -v /vendor/)
GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
BUILD_INFO_LDFLAGS=-ldflags "-extldflags '"-static"' -X main.buildDate=${BUILD_DATE} -X main.version=${v}"
GOLANG_CI = docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:v2.1.2 golangci-lint
.PHONY: test build

help:
	@echo "build      - go build"
	@echo "test       - go test"
	@echo "checkstyle - gofmt+golint+misspell"

test:
	$(GO) test -cover ${GODIRS_NOVENDOR}

lint:
	${GOLANG_CI} run ./...

fmt:
	${GOLANG_CI} fmt ./...

#build: checkstyle test
build:
	$(GO) build ${BUILD_INFO_LDFLAGS} -o ${BINARY_DIR}/gorp ./

clean:
	if [ -d ${BINARY_DIR} ] ; then rm -r ${BINARY_DIR} ; fi
	if [ -d 'build' ] ; then rm -r 'build' ; fi

tag:
	git tag -a v${v} -m "creating tag ${v}"
	git push origin "refs/tags/v${v}"

release:
	rm -rf dist
	goreleaser release

