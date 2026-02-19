.PHONY: run build test clean

run: build
	./hivemind $(ARGS)

build:
	go build -o hivemind .

test:
	go test ./... -v

clean:
	rm -f hivemind
