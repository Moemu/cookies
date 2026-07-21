.PHONY: build test vet fmt check migrate contract-check web-install web-check

build:
	go build ./cmd/cookies-api

migrate:
	go run ./cmd/cookies-migrate

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(go list -f '{{.Dir}}' ./... | xargs -n1 find -name '*.go' -type f)

check:
	@files="$$(gofmt -l $$(go list -f '{{.Dir}}' ./... | xargs -n1 find -name '*.go' -type f))"; test -z "$$files" || (echo "Unformatted Go files:"; echo "$$files"; exit 1)
	go vet ./...
	go test ./...
	npm run check --prefix web
	$(MAKE) contract-check

contract-check:
	npm run contract:check --prefix web

web-install:
	npm ci --prefix web

web-check:
	npm run check --prefix web
