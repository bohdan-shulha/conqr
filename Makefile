.PHONY: build test install clean

build:
	go build -o bin/conqr .

test:
	go test ./...

install:
	go install .

clean:
	rm -rf bin
