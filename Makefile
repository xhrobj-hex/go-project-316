.PHONY: build test run

build:
	mkdir -p bin
	go build -o bin/hexlet-go-crawler ./cmd/hexlet-go-crawler

test:
	go test ./...

run:
ifndef URL
	go run ./cmd/hexlet-go-crawler --help
else
	go run ./cmd/hexlet-go-crawler $(URL)
endif
