.PHONY: run build test clean

run: build
	./hivemind $(ARGS)

build:
	go build -o hivemind .
	go build -o hivemind-mcp ./cmd/mcp-server/

test:
	go test ./... -v

clean:
	rm -f hivemind hivemind-mcp
