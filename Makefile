build:
	@mkdir -p bin
	@go build -o bin/fs cmd/main.go

run: build
	@./bin/fs
