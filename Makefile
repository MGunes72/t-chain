GENESIS ?= genesis.json

build:
	cd cmd/t-chain && \
	go build -o ../../bin/t-chain

run: build
	./bin/t-chain -genesis=$(GENESIS)