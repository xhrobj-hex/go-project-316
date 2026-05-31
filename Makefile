.PHONY: build test lint run run-iv clean

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

run-iv:
	go run ./cmd/hexlet-go-crawler URL="--depth=1 --retries=0 https://irregularverbs.ru"

clean:
	rm -f ./bin/hexlet-go-crawler
