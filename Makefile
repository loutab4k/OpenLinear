.PHONY: fmt test vet check render validate dogfood dogfood-sync

DOGFOOD_DIR ?= examples/openlinear

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

# Dogfood: OpenLinear's own board (examples/openlinear).
dogfood:
	go run ./cmd/openlinear validate --data-dir $(DOGFOOD_DIR)
	go run ./cmd/openlinear render --data-dir $(DOGFOOD_DIR)

# Push the dogfood board to Telegram. Needs OPENLINEAR_BOT_TOKEN + OPENLINEAR_CHAT_ID.
dogfood-sync:
	go run ./cmd/openlinear sync --data-dir $(DOGFOOD_DIR)
