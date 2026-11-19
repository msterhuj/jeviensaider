BINARY  := benevole
PKG     := ./cmd/benevole
DB      ?= benevole.db
ADDR    ?= :8080
LDFLAGS := -s -w

.PHONY: all build run test race vet fmt generate clean build-arm64 build-armv7 backup

all: vet test build

build: generate
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY) $(PKG)

# Cross-compilation (pure Go SQLite driver: no C toolchain needed).
build-arm64: generate
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY)-linux-arm64 $(PKG)

build-armv7: generate
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY)-linux-armv7 $(PKG)

run: generate
	go run $(PKG) -addr $(ADDR) -db $(DB)

test: generate
	go test ./...

race: generate
	go test -race ./...

vet: generate
	go vet ./...

fmt:
	gofmt -w .
	go tool templ fmt internal/web

# Pages are templ components: generate their Go code (*_templ.go, not
# committed). Every target that compiles runs it first.
generate:
	go tool templ generate

# Consistent hot backup of the database (safe while the server runs).
backup:
	sqlite3 $(DB) ".backup 'backup-$$(date +%Y%m%d-%H%M%S).db'"

clean:
	rm -f $(BINARY) $(BINARY)-linux-*
