# Nostromo Makefile
VERSION ?= 1.0.0
MODULE  := github.com/gitstq/nostromo-relay
BIN     := bin
NAME    := nostromo
LDFLAGS := -s -w -X $(MODULE)/internal/server.Version=$(VERSION)

.PHONY: all build test race coverage fmt vet check clean release tidy docker

all: build

build:
	@mkdir -p $(BIN)
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN)/$(NAME) ./cmd/nostromo

# Cross-compile for all supported OS/arch pairs into bin/release.
release:
	@mkdir -p $(BIN)/release
	@for target in \
		linux/amd64 linux/arm64 \
		darwin/amd64 darwin/arm64 \
		windows/amd64 windows/arm64 ; do \
			os=$${target%/*}; arch=$${target#*/}; \
			out=$(BIN)/release/$(NAME)_$${os}_$${arch}; \
			if [ "$$os" = "windows" ]; then out=$$out.exe; fi; \
			echo "building $$out"; \
			GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -trimpath \
				-ldflags="$(LDFLAGS)" -o $$out ./cmd/nostromo || exit 1; \
		done
	cd $(BIN)/release && sha256sum * > checksums.txt && cat checksums.txt

test:
	go test ./...

race:
	go test -race ./...

coverage:
	go test -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -func=coverage.txt | tail -1

fmt:
	gofmt -s -w .

vet:
	go vet ./...

# CI gate: formatting must be clean and vet must pass.
check: vet
	@test -z "$$(gofmt -s -l .)" || (echo "gofmt needed:"; gofmt -s -l .; exit 1)
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf $(BIN) coverage.txt

docker:
	docker build -t nostromo:$(VERSION) .
