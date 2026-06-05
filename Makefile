.PHONY: build test test-race lint run run-iv clean

build:
	mkdir -p bin
	go build -o bin/hexlet-go-crawler ./cmd/hexlet-go-crawler

test:
	go test ./...

test-race:
	go test ./... -race -count=1

lint:
	golangci-lint run

run:
ifndef URL
	go run ./cmd/hexlet-go-crawler --help
else
	go run ./cmd/hexlet-go-crawler $(URL)
endif

run-iv:
	go run ./cmd/hexlet-go-crawler --depth=1 --retries=0 https://www.irregularverbs.ru

clean:
	rm -f ./bin/hexlet-go-crawler
