.PHONY: build test lint run clean

build:
	mkdir -p bin
	go build -o bin/hexlet-go-crawler ./cmd/hexlet-go-crawler

test:
	go test ./...

lint:
	golangci-lint run

run:
ifndef URL
	go run ./cmd/hexlet-go-crawler --help
else
	go run ./cmd/hexlet-go-crawler $(URL)
endif

clean:
	rm -f ./bin/hexlet-go-crawler
