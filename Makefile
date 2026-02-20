.PHONY: run build test clean install

run: build
	./hivemind $(ARGS)

build:
	go build -o hivemind .
	go build -o hivemind-mcp ./cmd/mcp-server/

install: build
	cp hivemind hivemind-mcp "$${DESTDIR:-/usr/local/bin}/"
	@echo "Installed to $${DESTDIR:-/usr/local/bin}/"

test:
	go test ./... -v

clean:
	rm -f hivemind hivemind-mcp
