.PHONY: fmt lint test test-integration

fmt:
	go fmt ./...

lint:
	@echo "lint not configured yet"

test:
	go test ./...

test-integration:
	@echo "integration tests not configured yet"
