.PHONY: run test race bench vet fmt build

run:
	go run ./cmd/hades

# All tests, verbose.
test:
	go test ./... -v

race:
	go test ./... -race

# Throughput numbers. No race detector: it distorts the measurement.
bench:
	go test ./internal/store -run '^$$' -bench . -benchmem

vet:
	go vet ./...

fmt:
	gofmt -l -w .

build:
	go build -o bin/hades ./cmd/hades
