build:
	cd cmd/chain && \
	go build -o ../../bin/chain

run: build
	./bin/chain -genesis=$(GENESIS)