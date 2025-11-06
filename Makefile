build:
	go build -o bin/testsnake -v ./cmd/testsnake

test:
	go test -v -race ./...
