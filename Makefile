.PHONY: fmt fmt-check test vet check render validate install dogfood dogfood-sync

DOGFOOD_DIR ?= examples/openlinear

fmt:
	gofmt -w ./cmd ./internal

fmt-check:
	test -z "$$(gofmt -l ./cmd ./internal)"

test:
	go test -race ./...

vet:
	go vet ./...

# The quality gate: CI runs exactly this, so local check == CI check.
check: fmt-check vet test validate

# Validate every example board (both boards from examples/boards.json).
validate:
	go run ./cmd/openlinear validate --data-dir examples/basic
	go run ./cmd/openlinear validate --data-dir examples/openlinear

render:
	go run ./cmd/openlinear render --data-dir examples/basic

# Install the CLI as `ol` into GOPATH/bin.
install:
	go build -trimpath -o "$$(go env GOPATH)/bin/ol" ./cmd/openlinear

# Dogfood: OpenLinear's own board (examples/openlinear).
dogfood:
	go run ./cmd/openlinear validate --data-dir $(DOGFOOD_DIR)
	go run ./cmd/openlinear render --data-dir $(DOGFOOD_DIR)

# Push the dogfood board to Telegram. Needs OPENLINEAR_BOT_TOKEN + OPENLINEAR_CHAT_ID.
dogfood-sync:
	go run ./cmd/openlinear sync --data-dir $(DOGFOOD_DIR)
