name		?= jukeboxjury

GO		?= go
DOCKER		?= docker
DOCKER_BUILDKIT ?= 1
VERSION		?= $(shell git log --pretty=format:%h -n 1)
BUILD_TIME	?= $(shell date)
# -s removes symbol table and -ldflags -w debugging symbols
LDFLAGS		?= -asmflags -trimpath -ldflags \
		   "-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}'"
GOARCH		?=
GOOS		?=
# CGO_ENABLED=0 == static by default
CGO_ENABLED	?= 0


all: test-unit lint build-all

_build: dist/$(APP_NAME)

dist/$(APP_NAME):
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) build $(LDFLAGS) \
		-o dist/$(APP_NAME) \
		main.go

.PHONY: clean
clean:
	rm -rf dist/

install-dependencies:
	@go get -d -v ./...

lint:
	@golangci-lint run ./...

vulncheck:
	@govulncheck ./...

escape-analysis:
	$(GO) build -gcflags="-m" 2>&1

# Easiest way to get proper profiler files:
# make -B LDFLAGS=-cover build-all
launch-profiler:
	$(GO) tool pprof -http=: cpu.prof

test-unit:
	go test -short -failfast -cover -race ./...

