.PHONY: all build test test-go test-ts check clean

BINDIR := bin

all: build

build:
	go build -o $(BINDIR)/fetch_open_markets ./cmd/fetch_open_markets
	go build -o $(BINDIR)/fetch_all_markets ./cmd/fetch_all_markets
	go build -o $(BINDIR)/server ./cmd/server
	go build -o $(BINDIR)/arb ./cmd/arb_scan

test: check

test-go:
	go test ./...

test-ts:
	npm run typecheck

check: test-go test-ts

clean:
	rm -rf $(BINDIR)
