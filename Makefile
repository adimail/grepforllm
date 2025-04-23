build:
	@mkdir -p bin
	@go build -o bin/fs main.go

run: build
	@./bin/fs
