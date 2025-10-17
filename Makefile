# use command for formatted code
lint:
	docker run -t --rm -v $$(pwd):/app -w /app golangci/golangci-lint:v2.5.0 golangci-lint run

# use command for build app
build:
	go mod vendor
	go build -ldflags "-w -s" -o mbt cmd/mbt/main.go