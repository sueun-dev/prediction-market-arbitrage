.PHONY: all build test clean

BINDIR := bin

all: build

build:
	go build -o $(BINDIR)/fetch_open_markets ./cmd/fetch_open_markets
	go build -o $(BINDIR)/fetch_all_markets ./cmd/fetch_all_markets
	go build -o $(BINDIR)/server ./cmd/server
	go build -o $(BINDIR)/arb ./cmd/arb_scan

test:
	go test ./...

clean:
	rm -rf $(BINDIR)
