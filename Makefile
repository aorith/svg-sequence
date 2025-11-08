SHELL := bash

.PHONY: test
test:
	@go test -v -timeout=5s -vet=all -count=1 ./...

.PHONY: fmt
fmt:
	@goimports -local $(shell go list -m) -w .
	@gofumpt -l -w .
