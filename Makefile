VERSION ?= 0.1.0
BINARY  := loadforge
LDFLAGS := -s -w -X github.com/xozan845-ctrl/loadforge-cli/internal/version.Version=$(VERSION)

.PHONY: build test dist clean mock fmt

build:
	mkdir -p dist
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY) ./cmd/loadforge

test:
	go test ./...

mock:
	go run ./cmd/mockserver

fmt:
	gofmt -w ./cmd ./internal

dist:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/loadforge-termux-arm64 ./cmd/loadforge
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/loadforge-linux-amd64 ./cmd/loadforge
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/loadforge-linux-arm64 ./cmd/loadforge
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/loadforge-windows-amd64.exe ./cmd/loadforge
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/loadforge-darwin-arm64 ./cmd/loadforge

clean:
	rm -rf dist
