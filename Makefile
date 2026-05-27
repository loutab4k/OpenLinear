.PHONY: fmt test vet check render validate

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

vet:
	go vet ./...

check: fmt test vet validate

validate:
	go run ./cmd/openlinear validate --data-dir examples/basic

render:
	go run ./cmd/openlinear render --data-dir examples/basic
